package dominio

import (
	"time"
)

// argentina es la hora en la que opera Rumbo; la usamos para mostrar la franja en el historial.
var argentina = time.FixedZone("ART", -3*60*60)

// Novedad es lo que informa el transporte sobre un traslado, ya traducido a nuestros estados.
type Novedad struct {
	TransferenciaID string
	IDRumbo         string
	Estado          Estado
	Franja          *Franja
	DemoraMinutos   int
	Motivo          string
	ActualizadoEn   time.Time
}

// Aplicar actualiza la transferencia con lo que informó el transporte y devuelve si cambió de estado.
// Una transferencia final no cambia más, y un dato más viejo que el último que aplicamos se ignora:
// así la transferencia nunca vuelve a un estado anterior. Uno con la misma fecha sí se aplica, porque
// Rumbo informa las fechas al segundo y en un mismo segundo puede haber dos cambios; si es el mismo
// dato repetido, no cambia nada.
func (t *Transferencia) Aplicar(n Novedad, ahora time.Time) bool {
	if t.ActualizadoEnRumbo != nil && n.ActualizadoEn.Before(*t.ActualizadoEnRumbo) {
		return false
	}
	actualizado := n.ActualizadoEn
	t.ActualizadoEnRumbo = &actualizado
	// Una transferencia que ya terminó no cambia más, pero guardamos la fecha igual: así un dato más
	// viejo que llegue después tampoco se toma como un traslado suelto.
	if t.Estado.EsFinal() {
		return false
	}

	if n.IDRumbo != "" {
		t.IDRumbo = n.IDRumbo
	}
	if n.Franja != nil {
		t.Franja = n.Franja
	}
	t.DemoraMinutos = n.DemoraMinutos
	if n.Motivo != "" {
		t.Motivo = n.Motivo
	}

	if n.Estado == t.Estado {
		return false
	}
	detalle := ""
	switch {
	case n.Estado == Programada && t.Franja != nil:
		detalle = "llega el " + t.Franja.Texto()
	case n.Estado.EsFinal() && n.Motivo != "":
		detalle = n.Motivo
	}
	t.CambiarEstado(n.Estado, detalle, ahora)
	return true
}

// Texto muestra la franja en hora de Argentina, por ejemplo "20/11 entre las 10:00 y las 12:00".
func (f Franja) Texto() string {
	return f.Inicio.In(argentina).Format("02/01") + " entre las " + f.Inicio.In(argentina).Format("15:04") +
		" y las " + f.Fin.In(argentina).Format("15:04")
}
