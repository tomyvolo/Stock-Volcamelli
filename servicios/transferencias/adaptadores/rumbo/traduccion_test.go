package rumbo

import (
	"encoding/json"
	"testing"

	"transferencias/dominio"
)

// Cada caso es una respuesta de Rumbo tomada de los ejemplos de su contrato (traslados.yaml),
// y el estado que tiene que quedar según la tabla de ADR-009.
func TestTraducir_EjemplosDelContrato(t *testing.T) {
	casos := []struct {
		nombre    string
		json      string
		estado    dominio.Estado
		comprobar func(n dominio.Novedad) bool
	}{
		{"programado", `{"id":"sol-7f3a9c","referencia_externa":"TRF-1","estado":"PROGRAMADO","franja":{"inicio":"2026-11-20T10:00:00-03:00","fin":"2026-11-20T12:00:00-03:00"},"actualizado_en":"2026-11-19T08:30:00-03:00"}`,
			dominio.Programada, func(n dominio.Novedad) bool {
				return n.IDRumbo == "sol-7f3a9c" && n.TransferenciaID == "TRF-1" && n.Franja != nil && n.Franja.Fin.Hour() == 12
			}},
		{"asignado", `{"id":"sol-7f3a9c","estado":"ASIGNADO","actualizado_en":"2026-11-19T17:05:00-03:00"}`,
			dominio.Programada, func(n dominio.Novedad) bool { return true }},
		{"en curso con demora", `{"id":"sol-7f3a9c","estado":"EN_CURSO","demora_minutos":15,"actualizado_en":"2026-11-20T10:30:00-03:00"}`,
			dominio.EnCurso, func(n dominio.Novedad) bool { return n.DemoraMinutos == 15 }},
		{"completado", `{"id":"sol-7f3a9c","estado":"COMPLETADO","demora_minutos":15,"finalizado_en":"2026-11-20T11:40:00-03:00","actualizado_en":"2026-11-20T11:40:00-03:00"}`,
			dominio.Completada, func(n dominio.Novedad) bool { return true }},
		{"cancelado tarde", `{"id":"sol-7f3a9c","estado":"CANCELADO","cancelacion":{"motivo":"La mercadería se despachó por otro medio.","tardia":true,"cancelada_en":"2026-11-20T07:30:00-03:00"},"actualizado_en":"2026-11-20T07:30:00-03:00"}`,
			dominio.Cancelada, func(n dominio.Novedad) bool { return n.Motivo == "La mercadería se despachó por otro medio." }},
		{"no realizado", `{"id":"sol-9b11d4","estado":"NO_REALIZADO","detalle_resultado":"La sucursal de origen estaba cerrada.","actualizado_en":"2026-11-21T08:25:00-03:00"}`,
			dominio.NoRealizada, func(n dominio.Novedad) bool { return n.Motivo == "La sucursal de origen estaba cerrada." }},
		{"interrumpido", `{"id":"sol-1","estado":"INTERRUMPIDO","detalle_resultado":"Avería del vehículo","actualizado_en":"2026-11-21T12:00:00-03:00"}`,
			dominio.NoRealizada, func(n dominio.Novedad) bool { return n.Motivo == "Avería del vehículo" }},
	}

	for _, caso := range casos {
		var tr traslado
		if err := json.Unmarshal([]byte(caso.json), &tr); err != nil {
			t.Fatalf("%s: el JSON del ejemplo no se pudo leer: %v", caso.nombre, err)
		}
		n, err := traducir(tr)
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
	_, err := traducir(traslado{ID: "sol-1", Estado: "EN_REVISION_ESPECIAL"})

	if err == nil {
		t.Error("Se esperaba un error por el estado desconocido")
	}
}
