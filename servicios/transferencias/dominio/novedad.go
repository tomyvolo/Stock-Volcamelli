package dominio

import (
	"fmt"
	"time"
)

// Novedad es lo que informa el transporte sobre un traslado, ya traducido a nuestros estados.
// Cuando el transporte propone un horario, Contrapropuesta es true: qué hacer depende de si
// el usuario había pedido una hora, y eso lo decide la transferencia, no el adaptador.
type Novedad struct {
	TransferenciaID  string
	IDRumbo          string
	Estado           Estado
	Contrapropuesta  bool
	HoraPropuesta    *time.Time
	VencePropuesta   *time.Time
	HoraComprometida *time.Time
	DemoraMinutos    int
	Motivo           string
	// La cancelación que pidió el usuario quedó esperando a que Rumbo la apruebe, o Rumbo no la aprobó.
	CancelacionPendiente bool
	CancelacionRechazada bool
	ActualizadoEn        time.Time
}

// Aplicar actualiza la transferencia con lo que informó el transporte y devuelve si cambió de estado.
// Una transferencia final no cambia más, y un dato más viejo que el último que aplicamos se ignora:
// así la transferencia nunca vuelve a un estado anterior. Uno con la misma fecha sí se aplica, porque
// Rumbo informa las fechas al segundo y en un mismo segundo puede haber dos cambios; si es el mismo
// dato repetido, no cambia nada.
func (t *Transferencia) Aplicar(n Novedad, ahora time.Time) bool {
	if t.Estado.EsFinal() {
		return false
	}
	if t.ActualizadoEnRumbo != nil && n.ActualizadoEn.Before(*t.ActualizadoEnRumbo) {
		return false
	}
	actualizado := n.ActualizadoEn
	t.ActualizadoEnRumbo = &actualizado

	if n.IDRumbo != "" {
		t.IDRumbo = n.IDRumbo
	}
	if n.HoraPropuesta != nil {
		t.HoraPropuesta, t.VencePropuesta = n.HoraPropuesta, n.VencePropuesta
	}
	if n.HoraComprometida != nil {
		t.HoraComprometida = n.HoraComprometida
	}
	t.DemoraMinutos = n.DemoraMinutos
	if n.Motivo != "" {
		t.Motivo = n.Motivo
	}

	nuevo := n.Estado
	detalle := ""
	switch {
	case n.Contrapropuesta && t.HoraLlegadaPedida == nil:
		// El usuario no pidió hora: le daba igual cuándo llegara, así que aceptamos solos.
		if t.RespuestaPorEnviar == nil {
			acepta := true
			t.RespuestaPorEnviar = &acepta
		}
		nuevo = Reservada
	case n.Contrapropuesta:
		// Pidió una hora y Rumbo propone otra: le preguntamos.
		nuevo = EsperandoConfirmacion
		if n.HoraPropuesta != nil {
			detalle = fmt.Sprintf("Rumbo propone que llegue el %s", n.HoraPropuesta.Format("02/01 a las 15:04"))
		}
	case n.CancelacionPendiente:
		nuevo = CancelacionPendiente
	}
	if n.CancelacionRechazada && t.Estado == CancelacionPendiente {
		t.CancelacionRechazada = true
		detalle = "Rumbo no aprobó la cancelación"
	}
	if nuevo.EsFinal() && n.Motivo != "" {
		detalle = n.Motivo
	}

	if nuevo == t.Estado {
		return false
	}
	t.CambiarEstado(nuevo, detalle, ahora)
	return true
}
