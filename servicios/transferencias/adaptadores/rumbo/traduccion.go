package rumbo

import (
	"fmt"
	"time"

	"transferencias/dominio"
)

// solicitud es la SolicitudTraslado del contrato de Rumbo, con los campos que usamos.
// Los campos que no conocemos se ignoran, como pide su contrato.
type solicitud struct {
	ID                string `json:"id"`
	ReferenciaExterna string `json:"referencia_externa"`
	Estado            string `json:"estado"`
	MotivoRechazo     string `json:"motivo_rechazo"`
	Contrapropuesta   *struct {
		HoraLlegadaPropuesta time.Time `json:"hora_llegada_propuesta"`
		VenceEn              time.Time `json:"vence_en"`
	} `json:"contrapropuesta"`
	Traslado *struct {
		Estado                  string    `json:"estado"`
		HoraLlegadaComprometida time.Time `json:"hora_llegada_comprometida"`
		DemoraMinutos           int       `json:"demora_minutos"`
		DetalleResultado        string    `json:"detalle_resultado"`
	} `json:"traslado"`
	Cancelacion *struct {
		Estado string `json:"estado"`
	} `json:"cancelacion"`
	ActualizadoEn time.Time `json:"actualizado_en"`
}

// traducir pasa una solicitud de Rumbo a una Novedad con nuestros estados (tabla de ADR-009).
// Si Rumbo informa un estado que no conocemos, devuelve error y la transferencia no cambia.
func traducir(s solicitud) (dominio.Novedad, error) {
	n := dominio.Novedad{TransferenciaID: s.ReferenciaExterna, IDRumbo: s.ID, ActualizadoEn: s.ActualizadoEn}

	switch s.Estado {
	case "RECIBIDA":
		n.Estado = dominio.Reservada
	case "CONTRAPROPUESTA":
		n.Estado = dominio.Reservada
		n.Contrapropuesta = true
		if s.Contrapropuesta != nil {
			hora, vence := s.Contrapropuesta.HoraLlegadaPropuesta, s.Contrapropuesta.VenceEn
			n.HoraPropuesta, n.VencePropuesta = &hora, &vence
		}
	case "RECHAZADA":
		n.Estado = dominio.Rechazada
		n.Motivo = s.MotivoRechazo
	case "RECHAZADA_POR_CLIENTE":
		n.Estado = dominio.Rechazada
		n.Motivo = "se rechazó el horario propuesto o no se respondió a tiempo"
	case "CANCELADA":
		n.Estado = dominio.Cancelada
	case "ACEPTADA":
		if s.Traslado == nil {
			return n, fmt.Errorf("Rumbo informó la solicitud %s ACEPTADA sin traslado", s.ID)
		}
		return traducirTraslado(s, n)
	default:
		return n, fmt.Errorf("Rumbo informó un estado desconocido en la solicitud %s: %s", s.ID, s.Estado)
	}
	return n, nil
}

func traducirTraslado(s solicitud, n dominio.Novedad) (dominio.Novedad, error) {
	hora := s.Traslado.HoraLlegadaComprometida
	n.HoraComprometida = &hora
	n.DemoraMinutos = s.Traslado.DemoraMinutos

	switch s.Traslado.Estado {
	case "PROGRAMADO", "ASIGNADO":
		n.Estado = dominio.Programada
	case "EN_CURSO":
		n.Estado = dominio.EnCurso
	case "COMPLETADO":
		n.Estado = dominio.Completada
	case "CANCELADO":
		n.Estado = dominio.Cancelada
	case "NO_REALIZADO", "INTERRUMPIDO":
		n.Estado = dominio.NoRealizada
		n.Motivo = s.Traslado.DetalleResultado
	default:
		return n, fmt.Errorf("Rumbo informó un estado de traslado desconocido en la solicitud %s: %s", s.ID, s.Traslado.Estado)
	}

	// Una cancelación tardía queda esperando que un operador de Rumbo la apruebe.
	if s.Cancelacion != nil && !n.Estado.EsFinal() {
		switch s.Cancelacion.Estado {
		case "PENDIENTE_APROBACION":
			n.CancelacionPendiente = true
		case "RECHAZADA":
			n.CancelacionRechazada = true
		}
	}
	return n, nil
}
