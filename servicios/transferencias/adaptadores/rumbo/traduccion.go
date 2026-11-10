package rumbo

import (
	"fmt"
	"time"

	"transferencias/dominio"
)

// traslado es el TrasladoReservado del contrato de Rumbo, con los campos que usamos.
// Los campos que no conocemos se ignoran, como pide su contrato.
type traslado struct {
	ID                string `json:"id"`
	ReferenciaExterna string `json:"referencia_externa"`
	Estado            string `json:"estado"`
	Franja            *struct {
		Inicio time.Time `json:"inicio"`
		Fin    time.Time `json:"fin"`
	} `json:"franja"`
	DemoraMinutos    int    `json:"demora_minutos"`
	DetalleResultado string `json:"detalle_resultado"`
	Cancelacion      *struct {
		Motivo string `json:"motivo"`
	} `json:"cancelacion"`
	ActualizadoEn time.Time `json:"actualizado_en"`
}

// traducir pasa un traslado de Rumbo a una Novedad con nuestros estados (tabla de ADR-009).
// Si Rumbo informa un estado que no conocemos, devuelve error y la transferencia no cambia.
func traducir(tr traslado) (dominio.Novedad, error) {
	n := dominio.Novedad{
		TransferenciaID: tr.ReferenciaExterna, IDRumbo: tr.ID, DemoraMinutos: tr.DemoraMinutos, ActualizadoEn: tr.ActualizadoEn,
	}
	if tr.Franja != nil {
		n.Franja = &dominio.Franja{Inicio: tr.Franja.Inicio, Fin: tr.Franja.Fin}
	}

	switch tr.Estado {
	case "PROGRAMADO", "ASIGNADO":
		n.Estado = dominio.Programada
	case "EN_CURSO":
		n.Estado = dominio.EnCurso
	case "COMPLETADO":
		n.Estado = dominio.Completada
	case "CANCELADO":
		n.Estado = dominio.Cancelada
		if tr.Cancelacion != nil {
			n.Motivo = tr.Cancelacion.Motivo
		}
	case "NO_REALIZADO", "INTERRUMPIDO":
		// Esta versión de Rumbo no los produce, pero están en su contrato.
		n.Estado = dominio.NoRealizada
		n.Motivo = tr.DetalleResultado
	default:
		return n, fmt.Errorf("Rumbo informó un estado desconocido en el traslado %s: %s", tr.ID, tr.Estado)
	}
	return n, nil
}
