package aplicacion

import (
	"fmt"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Versiones falsas de los puertos, para probar los casos de uso sin MongoDB, ni HTTP, ni Rumbo.

type repoFalso struct {
	transferencias map[string]*dominio.Transferencia
	ultimaConsulta time.Time
}

func (r *repoFalso) Guardar(t *dominio.Transferencia) error {
	r.transferencias[t.ID] = t
	return nil
}

func (r *repoFalso) Obtener(id string) (*dominio.Transferencia, error) {
	t, existe := r.transferencias[id]
	if !existe {
		return nil, puertos.ErrNoExiste
	}
	return t, nil
}

func (r *repoFalso) Listar(filtro puertos.Filtro) ([]*dominio.Transferencia, int64, error) {
	return nil, 0, nil
}

func (r *repoFalso) PendientesDeEnvio() ([]*dominio.Transferencia, error) {
	var lista []*dominio.Transferencia
	for _, t := range r.transferencias {
		lista = append(lista, t)
	}
	return lista, nil
}

func (r *repoFalso) UltimaConsulta() (time.Time, error) { return r.ultimaConsulta, nil }
func (r *repoFalso) GuardarUltimaConsulta(hasta time.Time) error {
	r.ultimaConsulta = hasta
	return nil
}

// inventarioFalso tiene el disponible de cada producto en cada sucursal ("producto-sucursal").
type inventarioFalso struct {
	disponible map[string]int
	errReserva error
	reservas   map[string]int
	liberadas  []string
}

func (i *inventarioFalso) NombreProducto(productoID uint, trackID string) (string, error) {
	if productoID == 99 {
		return "", puertos.ErrNoExiste
	}
	return "Harina 000", nil
}

func (i *inventarioFalso) Reservar(t *dominio.Transferencia, trackID string) error {
	if i.errReserva != nil {
		return i.errReserva
	}
	clave := fmt.Sprintf("%d-%d", t.ProductoID, t.OrigenID)
	if i.disponible[clave] < t.Cantidad {
		return puertos.ErrSinStock
	}
	i.disponible[clave] -= t.Cantidad
	i.reservas[t.ID] = t.Cantidad
	return nil
}

func (i *inventarioFalso) Liberar(transferenciaID, trackID string) error {
	i.liberadas = append(i.liberadas, transferenciaID)
	return nil
}

type sucursalesFalsas struct{}

func (sucursalesFalsas) Obtener(sucursalID uint, trackID string) (dominio.Ubicacion, error) {
	if sucursalID > 3 {
		return dominio.Ubicacion{}, puertos.ErrNoExiste
	}
	return dominio.Ubicacion{Nombre: fmt.Sprintf("Sucursal %d", sucursalID), Direccion: "Av. Colón 1250"}, nil
}

// transporteFalso responde lo que le cargue cada test.
type transporteFalso struct {
	respSolicitar  dominio.Novedad
	errSolicitar   error
	cambios        []dominio.Novedad
	respResponder  dominio.Novedad
	respondido     *bool
	respCancelar   dominio.Novedad
	errCancelar    error
	motivoCancelar string
}

func (f *transporteFalso) Solicitar(t *dominio.Transferencia) (dominio.Novedad, error) {
	n := f.respSolicitar
	n.TransferenciaID = t.ID
	return n, f.errSolicitar
}

func (f *transporteFalso) Cambios(desde time.Time) ([]dominio.Novedad, time.Time, error) {
	return f.cambios, desde.Add(time.Minute), nil
}

func (f *transporteFalso) Consultar(t *dominio.Transferencia) (dominio.Novedad, error) {
	return dominio.Novedad{}, puertos.ErrNoDisponible
}

func (f *transporteFalso) ResponderPropuesta(t *dominio.Transferencia, acepta bool) (dominio.Novedad, error) {
	f.respondido = &acepta
	return f.respResponder, nil
}

func (f *transporteFalso) Cancelar(t *dominio.Transferencia, motivo string) (dominio.Novedad, error) {
	f.motivoCancelar = motivo
	return f.respCancelar, f.errCancelar
}

type eventosFalsos struct {
	publicados []puertos.Evento
}

func (e *eventosFalsos) Publicar(evento puertos.Evento) error {
	e.publicados = append(e.publicados, evento)
	return nil
}

// tipos devuelve los tipos de evento publicados para una transferencia.
func (e *eventosFalsos) tipos(transferenciaID string) []string {
	var tipos []string
	for _, evento := range e.publicados {
		if evento.TransferenciaID == transferenciaID {
			tipos = append(tipos, evento.Tipo)
		}
	}
	return tipos
}

type registroFalso struct{}

func (registroFalso) Escribir(trackID, nivel, mensaje string) {}
