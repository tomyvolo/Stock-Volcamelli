package aplicacion

import (
	"errors"
	"fmt"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// PlazoParaEnviar es cuánto intentamos mandarle el pedido a Rumbo. Es lo que Rumbo guarda la
// Idempotency-Key: después, un reintento podría crear una solicitud repetida.
const PlazoParaEnviar = 24 * time.Hour

// EnviarPendientes manda lo que quedó por mandar: los pedidos nuevos a Rumbo, las respuestas a los
// horarios propuestos, las cancelaciones y las liberaciones de las transferencias fallidas.
// Lo que no se puede mandar ahora queda para la vuelta siguiente.
func (s *Servicio) EnviarPendientes() {
	pendientes, err := s.Repo.PendientesDeEnvio()
	if err != nil {
		s.Registro.Escribir("", "error", "no se pudieron leer las transferencias pendientes: "+err.Error())
		return
	}
	for _, pendiente := range pendientes {
		s.conLock(pendiente.ID, func(t *dominio.Transferencia) {
			s.liberar(t)
			s.solicitar(t)
			s.responder(t)
			s.cancelar(t)
		})
	}
}

// conLock lee la transferencia de nuevo con el lock tomado, por si otro la cambió desde que la buscamos,
// y le aplica el cambio.
func (s *Servicio) conLock(id string, cambio func(t *dominio.Transferencia)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.Repo.Obtener(id)
	if err != nil {
		s.Registro.Escribir("", "warn", fmt.Sprintf("no se pudo leer la transferencia %s: %v", id, err))
		return
	}
	cambio(t)
}

// ConsultarCambios trae de Rumbo lo que cambió desde la última consulta y lo aplica.
func (s *Servicio) ConsultarCambios() {
	desde, err := s.Repo.UltimaConsulta()
	if err != nil {
		s.Registro.Escribir("", "error", "no se pudo leer la última consulta a Rumbo: "+err.Error())
		return
	}
	novedades, hasta, err := s.Transporte.Cambios(desde)
	if err != nil {
		s.Registro.Escribir("", "warn", "no se pudieron consultar los cambios de Rumbo: "+err.Error())
		return
	}
	for _, n := range novedades {
		s.conLock(n.TransferenciaID, func(t *dominio.Transferencia) { s.aplicar(t, n) })
	}
	if err := s.Repo.GuardarUltimaConsulta(hasta); err != nil {
		s.Registro.Escribir("", "error", "no se pudo guardar la última consulta a Rumbo: "+err.Error())
	}
}

// liberar le pide a Inventario que libere la reserva de una transferencia fallida, hasta que responda.
func (s *Servicio) liberar(t *dominio.Transferencia) {
	if !t.LiberacionPorEnviar {
		return
	}
	if err := s.Inventario.Liberar(t.ID, t.TrackID); err != nil {
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("Inventario todavía no liberó la transferencia %s: %v", t.ID, err))
		return
	}
	t.LiberacionPorEnviar = false
	s.guardar(t)
	s.Registro.Escribir(t.TrackID, "info", "reserva de la transferencia fallida "+t.ID+" liberada")
}

// solicitar le manda el pedido a Rumbo. Si pasaron 24 horas sin poder, la transferencia queda RECHAZADA.
func (s *Servicio) solicitar(t *dominio.Transferencia) {
	if t.Estado != dominio.Reservada || t.EnviadaARumbo() {
		return
	}
	if s.Ahora().Sub(t.CreadaEn) >= PlazoParaEnviar {
		t.Motivo = "no se pudo mandar el pedido a Rumbo en 24 horas"
		t.CambiarEstado(dominio.Rechazada, t.Motivo, s.Ahora())
		s.guardarYPublicar(t)
		return
	}

	n, err := s.Transporte.Solicitar(t)
	switch {
	case err == nil:
		s.Registro.Escribir(t.TrackID, "info", fmt.Sprintf("transferencia %s enviada a Rumbo como %s", t.ID, n.IDRumbo))
		s.aplicar(t, n)
	case errors.Is(err, puertos.ErrRechazoDelPedido):
		t.Motivo = n.Motivo
		t.CambiarEstado(dominio.Rechazada, n.Motivo, s.Ahora())
		s.guardarYPublicar(t)
	default:
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("no se pudo mandar la transferencia %s a Rumbo, se reintenta: %v", t.ID, err))
	}
}

// responder le manda a Rumbo la respuesta al horario propuesto, del usuario o la automática.
func (s *Servicio) responder(t *dominio.Transferencia) {
	if t.RespuestaPorEnviar == nil || !t.EnviadaARumbo() || t.Estado.EsFinal() {
		return
	}
	n, err := s.Transporte.ResponderPropuesta(t, *t.RespuestaPorEnviar)
	s.despuesDeEnviar(t, n, err, func() { t.RespuestaPorEnviar = nil }, "la respuesta al horario")
}

// cancelar le manda a Rumbo la cancelación que pidió el usuario.
func (s *Servicio) cancelar(t *dominio.Transferencia) {
	if t.CancelacionPorEnviar == nil || !t.EnviadaARumbo() || t.Estado.EsFinal() {
		return
	}
	n, err := s.Transporte.Cancelar(t, *t.CancelacionPorEnviar)
	s.despuesDeEnviar(t, n, err, func() { t.CancelacionPorEnviar = nil }, "la cancelación")
}

// despuesDeEnviar aplica lo que respondió Rumbo. Si Rumbo dice que el estado ya no lo permite,
// consultamos la solicitud y nos quedamos con su estado actual. Si no respondió, se reintenta.
func (s *Servicio) despuesDeEnviar(t *dominio.Transferencia, n dominio.Novedad, err error, enviado func(), que string) {
	switch {
	case err == nil:
		enviado()
		s.aplicar(t, n)
	case errors.Is(err, puertos.ErrEstadoNoPermite), errors.Is(err, puertos.ErrRechazoDelPedido):
		enviado()
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("Rumbo no aceptó %s de la transferencia %s: %v", que, t.ID, err))
		actual, err := s.Transporte.Consultar(t)
		if err != nil {
			s.guardar(t)
			return
		}
		s.aplicar(t, actual)
	default:
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("no se pudo mandar %s de la transferencia %s, se reintenta: %v", que, t.ID, err))
	}
}

// aplicar actualiza la transferencia con lo que informó Rumbo, la guarda y, si cambió de estado, lo publica.
func (s *Servicio) aplicar(t *dominio.Transferencia, n dominio.Novedad) {
	anterior := t.Estado
	if !t.Aplicar(n, s.Ahora()) {
		s.guardar(t)
		return
	}
	s.Registro.Escribir(t.TrackID, "info", fmt.Sprintf("transferencia %s: %s -> %s", t.ID, anterior, t.Estado))
	s.guardarYPublicar(t)
}

func (s *Servicio) guardar(t *dominio.Transferencia) bool {
	if err := s.Repo.Guardar(t); err != nil {
		s.Registro.Escribir(t.TrackID, "error", fmt.Sprintf("no se pudo guardar la transferencia %s: %v", t.ID, err))
		return false
	}
	return true
}

// guardarYPublicar publica recién después de guardar: así un evento nunca anuncia algo que no quedó guardado.
func (s *Servicio) guardarYPublicar(t *dominio.Transferencia) {
	if s.guardar(t) {
		s.publicar(t)
	}
}
