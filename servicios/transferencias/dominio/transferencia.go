// Package dominio tiene la transferencia, sus estados y sus reglas.
// No conoce Gin, ni MongoDB, ni RabbitMQ, ni el formato de Rumbo.
package dominio

import (
	"errors"
	"time"
)

type Estado string

const (
	SinStock              Estado = "SIN_STOCK"
	Fallida               Estado = "FALLIDA"
	Reservada             Estado = "RESERVADA"
	EsperandoConfirmacion Estado = "ESPERANDO_CONFIRMACION"
	Programada            Estado = "PROGRAMADA"
	EnCurso               Estado = "EN_CURSO"
	Completada            Estado = "COMPLETADA"
	Rechazada             Estado = "RECHAZADA"
	CancelacionPendiente  Estado = "CANCELACION_PENDIENTE"
	Cancelada             Estado = "CANCELADA"
	NoRealizada           Estado = "NO_REALIZADA"
)

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
	ErrToleranciaMala   = errors.New("la tolerancia tiene que estar entre 0 y 720 minutos")
	ErrNoCancelable     = errors.New("la transferencia ya no se puede cancelar")
	ErrNoHayPropuesta   = errors.New("la transferencia no tiene un horario propuesto para responder")
	ErrPropuestaVencida = errors.New("el plazo para responder el horario propuesto ya venció")
)

// Ubicacion es lo que Rumbo necesita de cada sucursal.
type Ubicacion struct {
	Nombre    string
	Direccion string
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
	ToleranciaMinutos int
	Observaciones     string

	Estado    Estado
	Historial []CambioDeEstado

	// Lo que informa Rumbo.
	IDRumbo          string
	HoraPropuesta    *time.Time
	VencePropuesta   *time.Time
	HoraComprometida *time.Time
	DemoraMinutos    int
	Motivo           string
	// ActualizadoEnRumbo es la fecha del último dato de Rumbo que aplicamos. Un dato más viejo se ignora.
	ActualizadoEnRumbo *time.Time

	// Lo que todavía hay que mandar, en segundo plano.
	RespuestaPorEnviar   *bool   // respuesta al horario propuesto
	CancelacionPorEnviar *string // motivo de la cancelación
	LiberacionPorEnviar  bool    // pedirle a Inventario que libere la reserva (después de un timeout)
	CancelacionRechazada bool    // Rumbo no aprobó la última cancelación pendiente
}

// Nueva arma una transferencia y controla sus reglas: cantidad mayor a 0, sucursales distintas y hora futura.
func Nueva(id string, productoID uint, cantidad int, origenID, destinoID, creadaPor uint,
	horaPedida *time.Time, tolerancia int, observaciones string, ahora time.Time) (*Transferencia, error) {
	if cantidad <= 0 {
		return nil, ErrCantidadInvalida
	}
	if origenID == destinoID {
		return nil, ErrMismaSucursal
	}
	if horaPedida != nil && !horaPedida.After(ahora) {
		return nil, ErrHoraPasada
	}
	if tolerancia < 0 || tolerancia > 720 {
		return nil, ErrToleranciaMala
	}
	return &Transferencia{
		ID: id, ProductoID: productoID, Cantidad: cantidad, OrigenID: origenID, DestinoID: destinoID,
		CreadaPor: creadaPor, CreadaEn: ahora, HoraLlegadaPedida: horaPedida, ToleranciaMinutos: tolerancia,
		Observaciones: observaciones,
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

// EnviadaARumbo dice si Rumbo ya registró el pedido.
func (t *Transferencia) EnviadaARumbo() bool {
	return t.IDRumbo != ""
}

// PedirCancelacion registra que el usuario quiere cancelar. Si Rumbo todavía no tenía el pedido,
// se cancela en el momento y devuelve true; si no, queda para mandársela a Rumbo.
func (t *Transferencia) PedirCancelacion(motivo string, ahora time.Time) (canceladaYa bool, err error) {
	if t.Estado.EsFinal() || t.Estado == EnCurso || t.Estado == CancelacionPendiente || t.CancelacionPorEnviar != nil {
		return false, ErrNoCancelable
	}
	if !t.EnviadaARumbo() {
		t.Motivo = motivo
		t.CambiarEstado(Cancelada, "cancelada por el usuario antes de mandarla a Rumbo: "+motivo, ahora)
		return true, nil
	}
	t.CancelacionPorEnviar = &motivo
	t.CancelacionRechazada = false
	return false, nil
}

// ResponderPropuesta registra la respuesta del usuario al horario que propuso Rumbo.
func (t *Transferencia) ResponderPropuesta(acepta bool, ahora time.Time) error {
	if t.Estado != EsperandoConfirmacion || t.RespuestaPorEnviar != nil {
		return ErrNoHayPropuesta
	}
	if t.VencePropuesta != nil && !ahora.Before(*t.VencePropuesta) {
		return ErrPropuestaVencida
	}
	t.RespuestaPorEnviar = &acepta
	return nil
}
