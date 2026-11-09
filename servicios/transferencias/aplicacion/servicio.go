// Package aplicacion tiene los casos de uso de las transferencias. Usa el dominio y habla con
// afuera solo a través de los puertos.
package aplicacion

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

var (
	ErrSinPermiso          = errors.New("solo se pueden operar las transferencias que salen de la sucursal del usuario o llegan a ella")
	ErrNoExiste            = errors.New("la transferencia no existe")
	ErrSucursalInexistente = errors.New("la sucursal no existe")
	ErrProductoInexistente = errors.New("el producto no existe")
	ErrNoDisponible        = errors.New("un servicio que hace falta para crear la transferencia no respondió")
)

// Roles que pueden operar transferencias. El director puede con cualquiera; el resto, solo con las de su sucursal.
const rolDirector = "DIRECTOR"

// Usuario es quien hace el pedido, con los datos que el gateway sacó de su token.
type Usuario struct {
	ID         uint
	Rol        string
	SucursalID *uint
}

// Pedido es lo que carga el usuario para crear una transferencia.
type Pedido struct {
	ProductoID        uint
	Cantidad          int
	OrigenID          uint
	DestinoID         uint
	HoraLlegada       *time.Time
	ToleranciaMinutos int
	Observaciones     string
}

// Servicio tiene los casos de uso. Ahora y NuevoID se pueden reemplazar en los tests.
type Servicio struct {
	Repo       puertos.Repositorio
	Inventario puertos.Inventario
	Sucursales puertos.Sucursales
	Transporte puertos.Transporte
	Eventos    puertos.Eventos
	Registro   puertos.Registro
	Ahora      func() time.Time
	NuevoID    func() string

	// Los procesos de fondo y los pedidos de los usuarios pueden querer cambiar la misma transferencia
	// a la vez. Cada cambio la lee de nuevo y la guarda con este lock tomado, así ninguno pisa al otro.
	// Alcanza con un lock en memoria porque Transferencias corre en una sola instancia.
	mu sync.Mutex
}

func puedeOperar(u Usuario, origenID, destinoID uint) bool {
	if u.Rol == rolDirector {
		return true
	}
	return u.SucursalID != nil && (*u.SucursalID == origenID || *u.SucursalID == destinoID)
}

// Crear arma la transferencia, trae los datos de las sucursales y del producto, y reserva en el origen.
// El usuario ve el resultado enseguida: RESERVADA, SIN_STOCK o FALLIDA. El pedido a Rumbo sale en segundo plano.
func (s *Servicio) Crear(pedido Pedido, usuario Usuario, trackID string) (*dominio.Transferencia, error) {
	if !puedeOperar(usuario, pedido.OrigenID, pedido.DestinoID) {
		return nil, ErrSinPermiso
	}
	t, err := dominio.Nueva(s.NuevoID(), pedido.ProductoID, pedido.Cantidad, pedido.OrigenID, pedido.DestinoID, usuario.ID,
		pedido.HoraLlegada, pedido.ToleranciaMinutos, pedido.Observaciones, s.Ahora())
	if err != nil {
		return nil, err
	}
	t.TrackID = trackID

	if t.Origen, err = s.ubicacion(pedido.OrigenID, trackID); err != nil {
		return nil, err
	}
	if t.Destino, err = s.ubicacion(pedido.DestinoID, trackID); err != nil {
		return nil, err
	}
	t.NombreProducto, err = s.Inventario.NombreProducto(pedido.ProductoID, trackID)
	if errors.Is(err, puertos.ErrNoExiste) {
		return nil, ErrProductoInexistente
	}
	if err != nil {
		return nil, fmt.Errorf("%w: inventario: %v", ErrNoDisponible, err)
	}

	err = s.Inventario.Reservar(t, trackID)
	switch {
	case err == nil:
		t.CambiarEstado(dominio.Reservada, "", s.Ahora())
	case errors.Is(err, puertos.ErrSinStock):
		t.CambiarEstado(dominio.SinStock, "el origen no tenía stock disponible suficiente", s.Ahora())
	case errors.Is(err, puertos.ErrNoExiste):
		return nil, ErrProductoInexistente
	default:
		// No sabemos si Inventario llegó a reservar. No reintentamos: la transferencia queda FALLIDA
		// y un proceso de fondo le pide a Inventario que libere hasta que responda.
		t.LiberacionPorEnviar = true
		t.CambiarEstado(dominio.Fallida, "Inventario no respondió a tiempo", s.Ahora())
		s.Registro.Escribir(trackID, "error", fmt.Sprintf("transferencia %s fallida: no se pudo reservar: %v", t.ID, err))
	}

	if err := s.Repo.Guardar(t); err != nil {
		if t.Estado == dominio.Reservada {
			// Sin la transferencia guardada, nadie va a liberar esa reserva después: la liberamos ahora.
			s.Inventario.Liberar(t.ID, trackID)
		}
		return nil, err
	}
	s.publicar(t)
	return t, nil
}

func (s *Servicio) ubicacion(sucursalID uint, trackID string) (dominio.Ubicacion, error) {
	ubicacion, err := s.Sucursales.Obtener(sucursalID, trackID)
	if errors.Is(err, puertos.ErrNoExiste) {
		return dominio.Ubicacion{}, ErrSucursalInexistente
	}
	if err != nil {
		return dominio.Ubicacion{}, fmt.Errorf("%w: sucursales: %v", ErrNoDisponible, err)
	}
	return ubicacion, nil
}

func (s *Servicio) Obtener(id string) (*dominio.Transferencia, error) {
	t, err := s.Repo.Obtener(id)
	if errors.Is(err, puertos.ErrNoExiste) {
		return nil, ErrNoExiste
	}
	return t, err
}

func (s *Servicio) Listar(filtro puertos.Filtro) ([]*dominio.Transferencia, int64, error) {
	return s.Repo.Listar(filtro)
}

// Cancelar registra el pedido del usuario. Si Rumbo todavía no tenía el pedido, se cancela en el momento
// y se libera la reserva; si no, se le manda la cancelación a Rumbo en segundo plano.
func (s *Servicio) Cancelar(id string, usuario Usuario, motivo string) (*dominio.Transferencia, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.Obtener(id)
	if err != nil {
		return nil, err
	}
	if !puedeOperar(usuario, t.OrigenID, t.DestinoID) {
		return nil, ErrSinPermiso
	}
	canceladaYa, err := t.PedirCancelacion(motivo, s.Ahora())
	if err != nil {
		return nil, err
	}
	if err := s.Repo.Guardar(t); err != nil {
		return nil, err
	}
	if canceladaYa {
		s.publicar(t)
	}
	return t, nil
}

// Responder registra la respuesta del usuario al horario propuesto. Se le manda a Rumbo en segundo plano.
func (s *Servicio) Responder(id string, usuario Usuario, acepta bool) (*dominio.Transferencia, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.Obtener(id)
	if err != nil {
		return nil, err
	}
	if !puedeOperar(usuario, t.OrigenID, t.DestinoID) {
		return nil, ErrSinPermiso
	}
	if err := t.ResponderPropuesta(acepta, s.Ahora()); err != nil {
		return nil, err
	}
	if err := s.Repo.Guardar(t); err != nil {
		return nil, err
	}
	return t, nil
}

// publicar avisa cada cambio de estado y, si la transferencia terminó, le dice a Inventario
// que confirme o libere la reserva. Si no se puede publicar, queda en el log de error.
func (s *Servicio) publicar(t *dominio.Transferencia) {
	eventos := []puertos.Evento{{Tipo: puertos.TransferenciaActualizada}}
	switch {
	case t.Estado == dominio.Completada:
		eventos = append(eventos, puertos.Evento{Tipo: puertos.TrasladoCompletado})
	case t.Estado.LiberaLaReserva():
		eventos = append(eventos, puertos.Evento{Tipo: puertos.TrasladoCancelado, Motivo: t.Motivo})
	}
	for _, evento := range eventos {
		evento.ID = s.NuevoID()
		evento.TransferenciaID = t.ID
		evento.Estado = t.Estado
		evento.OrigenID = t.OrigenID
		evento.DestinoID = t.DestinoID
		evento.TrackID = t.TrackID
		if err := s.Eventos.Publicar(evento); err != nil {
			s.Registro.Escribir(t.TrackID, "error", fmt.Sprintf("no se pudo publicar %s de la transferencia %s: %v", evento.Tipo, t.ID, err))
		}
	}
}
