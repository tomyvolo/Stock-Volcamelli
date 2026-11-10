// Package dominio tiene la transferencia, sus estados y sus reglas.
// No conoce Gin, ni MongoDB, ni RabbitMQ, ni el formato de Rumbo.
package dominio

import (
	"errors"
	"time"
)

type Estado string

const (
	SinStock    Estado = "SIN_STOCK"
	Fallida     Estado = "FALLIDA"
	Reservada   Estado = "RESERVADA"
	Programada  Estado = "PROGRAMADA"
	EnCurso     Estado = "EN_CURSO"
	Completada  Estado = "COMPLETADA"
	Rechazada   Estado = "RECHAZADA"
	Cancelada   Estado = "CANCELADA"
	NoRealizada Estado = "NO_REALIZADA"
)

// DiasReservables es hasta cuántos días adelante se puede reservar un traslado en Rumbo.
const DiasReservables = 30

// EsFinal dice si la transferencia ya no cambia más.
func (e Estado) EsFinal() bool {
	switch e {
	case SinStock, Fallida, Completada, Rechazada, Cancelada, NoRealizada:
		return true
	}
	return false
}

// LiberaLaReserva dice si al llegar a este estado las unidades vuelven al origen.
func (e Estado) LiberaLaReserva() bool {
	return e == Rechazada || e == Cancelada || e == NoRealizada
}

var (
	ErrCantidadInvalida = errors.New("la cantidad tiene que ser mayor a 0")
	ErrMismaSucursal    = errors.New("el origen y el destino tienen que ser sucursales distintas")
	ErrHoraPasada       = errors.New("la hora de llegada tiene que ser futura")
	ErrHoraLejana       = errors.New("la hora de llegada tiene que estar dentro de los próximos 30 días")
	ErrNoCancelable     = errors.New("la transferencia ya no se puede cancelar")
)

// Ubicacion es lo que Rumbo necesita de cada sucursal.
type Ubicacion struct {
	Nombre    string
	Direccion string
}

// Franja es la ventana de 2 horas en la que el traslado llega al destino.
type Franja struct {
	Inicio time.Time
	Fin    time.Time
}

// CambioDeEstado es una entrada del historial.
type CambioDeEstado struct {
	Estado  Estado
	Fecha   time.Time
	Detalle string
}

// Transferencia es lo que pidió el usuario, su estado y lo que fue informando Rumbo.
type Transferencia struct {
	ID             string
	ProductoID     uint
	NombreProducto string
	Cantidad       int
	OrigenID       uint
	DestinoID      uint
	Origen         Ubicacion
	Destino        Ubicacion
	CreadaPor      uint
	CreadaEn       time.Time
	// TrackID es el del pedido que la creó. Lo usan los procesos de fondo, así toda su vida queda en la misma traza.
	TrackID string

	// Opcionales que elige el usuario.
	HoraLlegadaPedida *time.Time
	Observaciones     string

	Estado    Estado
	Historial []CambioDeEstado

	// Lo que informa Rumbo.
	IDRumbo string
	// Franja es la que elegimos para reservar y, cuando Rumbo la reserva, en la que se compromete a llegar.
	// La guardamos antes de mandar el pedido: si no sabemos si llegó, el reintento va con la misma.
	Franja        *Franja
	DemoraMinutos int
	Motivo        string
	// ActualizadoEnRumbo es la fecha del último dato de Rumbo que aplicamos. Un dato más viejo se ignora.
	ActualizadoEnRumbo *time.Time

	// Lo que todavía hay que mandar, en segundo plano.
	CancelacionPorEnviar *string // motivo de la cancelación
	LiberacionPorEnviar  bool    // pedirle a Inventario que libere la reserva (después de un timeout)
}

// Nueva arma una transferencia y controla sus reglas: cantidad mayor a 0, sucursales distintas y una hora
// de llegada futura que Rumbo pueda reservar.
func Nueva(id string, productoID uint, cantidad int, origenID, destinoID, creadaPor uint,
	horaPedida *time.Time, observaciones string, ahora time.Time) (*Transferencia, error) {
	if cantidad <= 0 {
		return nil, ErrCantidadInvalida
	}
	if origenID == destinoID {
		return nil, ErrMismaSucursal
	}
	if horaPedida != nil && !horaPedida.After(ahora) {
		return nil, ErrHoraPasada
	}
	if horaPedida != nil && horaPedida.After(ahora.AddDate(0, 0, DiasReservables)) {
		return nil, ErrHoraLejana
	}
	return &Transferencia{
		ID: id, ProductoID: productoID, Cantidad: cantidad, OrigenID: origenID, DestinoID: destinoID,
		CreadaPor: creadaPor, CreadaEn: ahora, HoraLlegadaPedida: horaPedida, Observaciones: observaciones,
	}, nil
}

// CambiarEstado pasa la transferencia a otro estado y lo anota en el historial.
func (t *Transferencia) CambiarEstado(nuevo Estado, detalle string, ahora time.Time) {
	t.Estado = nuevo
	t.Historial = append(t.Historial, CambioDeEstado{Estado: nuevo, Fecha: ahora, Detalle: detalle})
}

// PasaPorSucursal dice si la transferencia sale de la sucursal o llega a ella.
func (t *Transferencia) PasaPorSucursal(sucursalID uint) bool {
	return t.OrigenID == sucursalID || t.DestinoID == sucursalID
}

// EnviadaARumbo dice si Rumbo ya reservó el traslado.
func (t *Transferencia) EnviadaARumbo() bool {
	return t.IDRumbo != ""
}

// PedirCancelacion registra que el usuario quiere cancelar. Si Rumbo todavía no tenía el pedido,
// se cancela en el momento y devuelve true; si no, queda para mandársela a Rumbo.
func (t *Transferencia) PedirCancelacion(motivo string, ahora time.Time) (canceladaYa bool, err error) {
	if t.Estado.EsFinal() || t.Estado == EnCurso || t.CancelacionPorEnviar != nil {
		return false, ErrNoCancelable
	}
	if !t.EnviadaARumbo() {
		t.Motivo = motivo
		t.CambiarEstado(Cancelada, "cancelada por el usuario antes de reservar el traslado: "+motivo, ahora)
		return true, nil
	}
	t.CancelacionPorEnviar = &motivo
	return false, nil
}
