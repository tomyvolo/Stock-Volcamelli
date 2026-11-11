package aplicacion

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Versiones falsas de los puertos, para probar los casos de uso sin MongoDB, ni HTTP, ni Rumbo.

// El mutex es porque los eventos se publican en segundo plano y, al terminar, guardan la transferencia
// mientras el proceso de fondo puede estar recorriendo las pendientes.
type repoFalso struct {
	mu             sync.Mutex
	transferencias map[string]*dominio.Transferencia
	ultimaConsulta time.Time
	errGuardar     error // simula MongoDB fallando al guardar
}

func (r *repoFalso) Guardar(t *dominio.Transferencia) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errGuardar != nil {
		return r.errGuardar
	}
	r.transferencias[t.ID] = t
	return nil
}

func (r *repoFalso) Obtener(id string) (*dominio.Transferencia, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
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
	// franjas con lugar de cada día ("2026-11-02").
	franjas         map[string][]dominio.Franja
	diasConsultados int
	// errReservar son los errores que devuelven las próximas reservas, en orden; errSiempre, todas.
	errReservar    []error
	errSiempre     error
	motivoRechazo  string
	reservadas     []dominio.Franja
	respReservar   dominio.Novedad
	cambios        []dominio.Novedad
	respConsultar  dominio.Novedad
	respCancelar   dominio.Novedad
	errCancelar    error
	motivoCancelar string
	canceladoEn    string // id de Rumbo del traslado que se mandó a cancelar
}

func (f *transporteFalso) Franjas(fecha time.Time) ([]dominio.Franja, error) {
	f.diasConsultados++
	return f.franjas[fecha.Format("2006-01-02")], nil
}

func (f *transporteFalso) Reservar(t *dominio.Transferencia) (dominio.Novedad, error) {
	f.reservadas = append(f.reservadas, *t.Franja)
	if f.errSiempre != nil {
		return dominio.Novedad{}, f.errSiempre
	}
	if len(f.errReservar) > 0 {
		err := f.errReservar[0]
		f.errReservar = f.errReservar[1:]
		return dominio.Novedad{Motivo: f.motivoRechazo}, err
	}
	n := f.respReservar
	n.TransferenciaID = t.ID
	n.Franja = t.Franja
	return n, nil
}

func (f *transporteFalso) Cambios(desde time.Time) ([]dominio.Novedad, time.Time, error) {
	return f.cambios, desde.Add(time.Minute), nil
}

func (f *transporteFalso) Consultar(t *dominio.Transferencia) (dominio.Novedad, error) {
	return f.respConsultar, nil
}

func (f *transporteFalso) Cancelar(t *dominio.Transferencia, motivo string) (dominio.Novedad, error) {
	f.motivoCancelar = motivo
	f.canceladoEn = t.IDRumbo
	return f.respCancelar, f.errCancelar
}

type eventosFalsos struct {
	mu         sync.Mutex
	publicados []puertos.Evento
	espera     *sync.WaitGroup // los eventos que el servicio publica en segundo plano
	lento      time.Duration   // simula RabbitMQ caído, que tarda en reintentos
	caido      bool            // simula RabbitMQ caído después de los reintentos: Publicar falla
	intentos   int
}

func (e *eventosFalsos) Publicar(evento puertos.Evento) error {
	time.Sleep(e.lento)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.intentos++
	if e.caido {
		return errors.New("RabbitMQ no responde")
	}
	e.publicados = append(e.publicados, evento)
	return nil
}

// rabbitMQ espera los eventos en segundo plano y deja a RabbitMQ caído o andando.
func (e *eventosFalsos) rabbitMQ(caido bool) {
	e.espera.Wait()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.caido = caido
	e.intentos = 0
}

// todos espera los eventos en segundo plano y devuelve todo lo publicado.
func (e *eventosFalsos) todos() []puertos.Evento {
	e.espera.Wait()
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.publicados)
}

// tipos devuelve los tipos de evento publicados para una transferencia.
func (e *eventosFalsos) tipos(transferenciaID string) []string {
	var tipos []string
	for _, evento := range e.todos() {
		if evento.TransferenciaID == transferenciaID {
			tipos = append(tipos, evento.Tipo)
		}
	}
	return tipos
}

type registroFalso struct{}

func (registroFalso) Escribir(trackID, nivel, mensaje string) {}
