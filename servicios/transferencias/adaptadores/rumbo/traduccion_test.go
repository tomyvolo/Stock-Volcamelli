package rumbo

import (
	"encoding/json"
	"testing"

	"transferencias/dominio"
)

// Cada caso es una respuesta de Rumbo tomada de los ejemplos de su contrato (traslados-v2.yaml),
// y el estado que tiene que quedar según la tabla de ADR-009.
func TestTraducir_EjemplosDelContrato(t *testing.T) {
	casos := []struct {
		nombre    string
		json      string
		estado    dominio.Estado
		comprobar func(n dominio.Novedad) bool
	}{
		{"recibida", `{"id":"sol-7f3a9c","referencia_externa":"TRF-1","estado":"RECIBIDA","actualizado_en":"2026-10-15T08:30:00-03:00"}`,
			dominio.Reservada, func(n dominio.Novedad) bool { return n.IDRumbo == "sol-7f3a9c" && n.TransferenciaID == "TRF-1" }},
		{"contrapropuesta", `{"id":"sol-7f3a9c","estado":"CONTRAPROPUESTA","contrapropuesta":{"hora_llegada_propuesta":"2026-10-16T17:00:00-03:00","vence_en":"2026-10-16T09:10:00-03:00"},"actualizado_en":"2026-10-15T09:10:00-03:00"}`,
			dominio.Reservada, func(n dominio.Novedad) bool {
				return n.Contrapropuesta && n.HoraPropuesta != nil && n.VencePropuesta != nil
			}},
		{"aceptada y programada", `{"id":"sol-7f3a9c","estado":"ACEPTADA","traslado":{"id":"tra-51c2e0","estado":"PROGRAMADO","hora_llegada_comprometida":"2026-10-16T17:00:00-03:00"},"actualizado_en":"2026-10-15T09:40:00-03:00"}`,
			dominio.Programada, func(n dominio.Novedad) bool { return n.HoraComprometida != nil }},
		{"completada con demora", `{"id":"sol-7f3a9c","estado":"ACEPTADA","traslado":{"id":"tra-51c2e0","estado":"COMPLETADO","hora_llegada_comprometida":"2026-10-16T14:00:00-03:00","demora_minutos":15},"actualizado_en":"2026-10-16T14:15:00-03:00"}`,
			dominio.Completada, func(n dominio.Novedad) bool { return n.DemoraMinutos == 15 }},
		{"rechazada", `{"id":"sol-9b11d4","estado":"RECHAZADA","motivo_rechazo":"No hay vehículos con capacidad disponible para esa fecha.","actualizado_en":"2026-10-15T12:20:00-03:00"}`,
			dominio.Rechazada, func(n dominio.Novedad) bool {
				return n.Motivo == "No hay vehículos con capacidad disponible para esa fecha."
			}},
		{"rechazada por el cliente", `{"id":"sol-1","estado":"RECHAZADA_POR_CLIENTE","actualizado_en":"2026-10-15T12:20:00-03:00"}`,
			dominio.Rechazada, func(n dominio.Novedad) bool { return n.Motivo != "" }},
		{"cancelación pendiente", `{"id":"sol-7f3a9c","estado":"ACEPTADA","traslado":{"id":"tra-51c2e0","estado":"ASIGNADO","hora_llegada_comprometida":"2026-10-16T14:00:00-03:00"},"cancelacion":{"estado":"PENDIENTE_APROBACION","solicitada_en":"2026-10-16T07:30:00-03:00"},"actualizado_en":"2026-10-16T07:30:00-03:00"}`,
			dominio.Programada, func(n dominio.Novedad) bool { return n.CancelacionPendiente }},
		{"traslado interrumpido", `{"id":"sol-1","estado":"ACEPTADA","traslado":{"id":"tra-1","estado":"INTERRUMPIDO","hora_llegada_comprometida":"2026-10-16T14:00:00-03:00","detalle_resultado":"Avería del vehículo"},"actualizado_en":"2026-10-16T12:00:00-03:00"}`,
			dominio.NoRealizada, func(n dominio.Novedad) bool { return n.Motivo == "Avería del vehículo" }},
	}

	for _, caso := range casos {
		var s solicitud
		if err := json.Unmarshal([]byte(caso.json), &s); err != nil {
			t.Fatalf("%s: el JSON del ejemplo no se pudo leer: %v", caso.nombre, err)
		}
		n, err := traducir(s)
		if err != nil {
			t.Errorf("%s: no se esperaba un error, pero falló: %v", caso.nombre, err)
			continue
		}
		if n.Estado != caso.estado || !caso.comprobar(n) {
			t.Errorf("%s: se esperaba %s, pero se obtuvo %+v", caso.nombre, caso.estado, n)
		}
	}
}

// Un estado que no está en la tabla no cambia la transferencia: devuelve error para dejarlo en el log.
func TestTraducir_EstadoDesconocido(t *testing.T) {
	_, err := traducir(solicitud{ID: "sol-1", Estado: "EN_REVISION_ESPECIAL"})

	if err == nil {
		t.Error("Se esperaba un error por el estado desconocido")
	}
}
