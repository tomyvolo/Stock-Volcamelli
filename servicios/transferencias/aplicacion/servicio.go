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
	ProductoID    uint
	Cantidad      int
	OrigenID      uint
	DestinoID     uint
	HoraLlegada   *time.Time
	Observaciones string
}

// Servicio tiene los casos de uso. Ahora y NuevoID se pueden reemplazar en los tests. Transporte puede
// ser nil: sin Rumbo configurado, las transferencias se crean y quedan RESERVADA (ADR-009).
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
	// a la vez. Cada cambio la lee de nuevo y la guarda con el lock de esa transferencia tomado, así
	// ninguno pisa al otro. Es un lock por transferencia: mientras el proceso de fondo espera a Rumbo
	// por una, las demás se pueden cancelar sin esperar. Alcanza con locks en memoria porque
	// Transferencias corre en una sola instancia.
	locks sync.Map // id de la transferencia -> *sync.Mutex

	// Los eventos de los pedidos de los usuarios se publican en segundo plano. Los tests esperan acá a que terminen.
	publicando sync.WaitGroup
	// enVuelo son las transferencias cuyos eventos se están publicando en segundo plano. El proceso de
	// fondo no las vuelve a publicar mientras tanto.
	enVuelo sync.Map // id de la transferencia -> true
}

// bloquear toma el lock de la transferencia y devuelve la función que lo suelta.
func (s *Servicio) bloquear(id string) func() {
	lock, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func puedeOperar(u Usuario, origenID, destinoID uint) bool {
	if u.Rol == rolDirector {
		return true
	}
	return u.SucursalID != nil && (*u.SucursalID == origenID || *u.SucursalID == destinoID)
}

// Crear arma la transferencia, trae los datos de las sucursales y del producto, y reserva en el origen.
// El usuario ve el resultado enseguida: RESERVADA, SIN_STOCK o FALLIDA. La reserva en Rumbo sale en segundo plano.
func (s *Servicio) Crear(pedido Pedido, usuario Usuario, trackID string) (*dominio.Transferencia, error) {
	if !puedeOperar(usuario, pedido.OrigenID, pedido.DestinoID) {
		return nil, ErrSinPermiso
	}
	t, err := dominio.Nueva(s.NuevoID(), pedido.ProductoID, pedido.Cantidad, pedido.OrigenID, pedido.DestinoID, usuario.ID,
		pedido.HoraLlegada, pedido.Observaciones, s.Ahora())
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

	t.EventosPorPublicar = true
	if err := s.Repo.Guardar(t); err != nil {
		if t.Estado == dominio.Reservada {
			// Sin la transferencia guardada, nadie va a liberar esa reserva después: la liberamos ahora.
			s.Inventario.Liberar(t.ID, trackID)
		}
		return nil, err
	}
	s.publicarSinEsperar(t)
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

// Cancelar registra el pedido del usuario. Si Rumbo todavía no reservó el traslado, se cancela en el momento
// y se libera la reserva; si no, se le manda la cancelación a Rumbo en segundo plano.
func (s *Servicio) Cancelar(id string, usuario Usuario, motivo string) (*dominio.Transferencia, error) {
	defer s.bloquear(id)()
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
	if canceladaYa {
		t.EventosPorPublicar = true
	}
	if err := s.Repo.Guardar(t); err != nil {
		return nil, err
	}
	if canceladaYa {
		s.publicarSinEsperar(t)
	}
	return t, nil
}

// publicarSinEsperar avisa cada cambio de estado y, si la transferencia terminó, le dice a Inventario
// que confirme o libere la reserva. Si salieron todos los eventos, saca la marca; si no, la marca queda
// y el proceso de fondo los vuelve a publicar.
//
// Publica en segundo plano y sin el lock de la transferencia. Con RabbitMQ caído, cada evento tarda
// hasta 7 segundos en reintentos y el gateway corta a los 5: si el usuario esperara la publicación, o
// esperara el lock mientras otro publica, vería un error aunque su pedido ya estaba guardado.
func (s *Servicio) publicarSinEsperar(t *dominio.Transferencia) {
	eventos, id, estado := s.eventos(t), t.ID, t.Estado
	s.enVuelo.Store(id, true)
	s.publicando.Add(1)
	go func() {
		defer s.publicando.Done()
		defer s.enVuelo.Delete(id)
		if s.enviar(eventos) {
			s.conLock(id, func(t *dominio.Transferencia) { s.publicado(t, estado) })
		}
	}()
}

// publicado saca la marca de eventos por publicar, si la transferencia sigue en el estado que se publicó.
// Si cambió mientras tanto, la marca es del cambio nuevo y queda.
func (s *Servicio) publicado(t *dominio.Transferencia, estado dominio.Estado) {
	if t.EventosPorPublicar && t.Estado == estado {
		t.EventosPorPublicar = false
		s.guardar(t)
	}
}

func (s *Servicio) eventos(t *dominio.Transferencia) []puertos.Evento {
	eventos := []puertos.Evento{{Tipo: puertos.TransferenciaActualizada}}
	switch {
	case t.Estado == dominio.Completada:
		eventos = append(eventos, puertos.Evento{Tipo: puertos.TrasladoCompletado})
	case t.Estado.LiberaLaReserva():
		eventos = append(eventos, puertos.Evento{Tipo: puertos.TrasladoCancelado, Motivo: t.Motivo})
	}
	for i := range eventos {
		eventos[i].ID = s.NuevoID()
		eventos[i].TransferenciaID = t.ID
		eventos[i].Estado = t.Estado
		eventos[i].OrigenID = t.OrigenID
		eventos[i].DestinoID = t.DestinoID
		eventos[i].TrackID = t.TrackID
	}
	return eventos
}

// enviar publica los eventos en orden y devuelve false si alguno no salió. Ante el primero que falla
// deja de intentar: RabbitMQ no está, y el proceso de fondo los vuelve a publicar todos.
func (s *Servicio) enviar(eventos []puertos.Evento) bool {
	for _, evento := range eventos {
		if err := s.Eventos.Publicar(evento); err != nil {
			s.Registro.Escribir(evento.TrackID, "error", fmt.Sprintf("no se pudo publicar %s de la transferencia %s, se reintenta: %v", evento.Tipo, evento.TransferenciaID, err))
			return false
		}
	}
	return true
}
