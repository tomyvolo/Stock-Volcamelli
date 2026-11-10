package aplicacion

import (
	"errors"
	"fmt"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// PlazoParaReservar es cuánto intentamos reservar el traslado en Rumbo. Después, la transferencia queda
// RECHAZADA y se libera la reserva.
const PlazoParaReservar = 24 * time.Hour

// EnviarPendientes manda lo que quedó por mandar: las reservas nuevas a Rumbo, las cancelaciones y las
// liberaciones de las transferencias fallidas. Lo que no se puede mandar ahora queda para la vuelta siguiente.
// Sin Rumbo configurado, solo libera.
func (s *Servicio) EnviarPendientes() {
	pendientes, err := s.Repo.PendientesDeEnvio()
	if err != nil {
		s.Registro.Escribir("", "error", "no se pudieron leer las transferencias pendientes: "+err.Error())
		return
	}
	for _, pendiente := range pendientes {
		s.conLock(pendiente.ID, func(t *dominio.Transferencia) {
			s.liberar(t)
			if s.Transporte != nil {
				s.reservar(t)
				s.cancelar(t)
			}
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

// reservar elige la franja, si todavía no tiene una, y la reserva en Rumbo. Si pasaron 24 horas sin
// poder, la transferencia queda RECHAZADA.
func (s *Servicio) reservar(t *dominio.Transferencia) {
	if t.Estado != dominio.Reservada || t.EnviadaARumbo() {
		return
	}
	if s.Ahora().Sub(t.CreadaEn) >= PlazoParaReservar {
		s.rechazar(t, "no se pudo reservar el traslado en Rumbo en 24 horas")
		return
	}

	if t.Franja == nil {
		franja, err := s.elegirFranja(t)
		if err != nil {
			s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("no se pudo consultar la disponibilidad de Rumbo para la transferencia %s, se reintenta: %v", t.ID, err))
			return
		}
		if franja == nil {
			s.rechazar(t, fmt.Sprintf("Rumbo no tiene lugar en los próximos %d días", dominio.DiasReservables))
			return
		}
		// La guardamos antes de mandarla: si no nos enteramos de si Rumbo la reservó, el reintento va
		// con la misma franja y la misma clave, y Rumbo no reserva dos veces.
		t.Franja = franja
		if !s.guardar(t) {
			return
		}
	}

	n, err := s.Transporte.Reservar(t)
	switch {
	case err == nil:
		s.Registro.Escribir(t.TrackID, "info", fmt.Sprintf("transferencia %s reservada en Rumbo como %s", t.ID, n.IDRumbo))
		s.aplicar(t, n)
	case errors.Is(err, puertos.ErrSinLugar):
		// Otro grupo tomó el último lugar o ya es tarde para esa franja: en la vuelta siguiente buscamos otra.
		s.Registro.Escribir(t.TrackID, "info", fmt.Sprintf("la franja del %s ya no sirve para la transferencia %s, buscamos otra: %v", t.Franja.Texto(), t.ID, err))
		t.Franja = nil
		s.guardar(t)
	case errors.Is(err, puertos.ErrRechazoDelPedido):
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("Rumbo rechazó la transferencia %s: %v", t.ID, err))
		s.rechazar(t, n.Motivo)
	default:
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("no se pudo reservar la transferencia %s en Rumbo, se reintenta: %v", t.ID, err))
	}
}

// elegirFranja busca la franja que contiene la hora pedida o, si no tiene lugar o no se pidió hora, la
// primera con lugar después. Recorre la disponibilidad día por día, hasta los días que permite Rumbo.
// Devuelve nil si no hay lugar en ninguno.
func (s *Servicio) elegirFranja(t *dominio.Transferencia) (*dominio.Franja, error) {
	desde := s.Ahora()
	if t.HoraLlegadaPedida != nil && t.HoraLlegadaPedida.After(desde) {
		desde = *t.HoraLlegadaPedida
	}
	limite := s.Ahora().AddDate(0, 0, dominio.DiasReservables)
	for dia := desde; !dia.After(limite); dia = dia.AddDate(0, 0, 1) {
		franjas, err := s.Transporte.Franjas(dia)
		if err != nil {
			return nil, err
		}
		for _, franja := range franjas {
			if franja.Fin.After(desde) {
				return &franja, nil
			}
		}
	}
	return nil, nil
}

func (s *Servicio) rechazar(t *dominio.Transferencia, motivo string) {
	t.Motivo = motivo
	t.CambiarEstado(dominio.Rechazada, motivo, s.Ahora())
	s.guardarYPublicar(t)
}

// cancelar le manda a Rumbo la cancelación que pidió el usuario, o la de un traslado suelto.
func (s *Servicio) cancelar(t *dominio.Transferencia) {
	if t.CancelacionPorEnviar == nil || !t.EnviadaARumbo() {
		return
	}
	n, err := s.Transporte.Cancelar(t, *t.CancelacionPorEnviar)
	switch {
	case err == nil:
		t.CancelacionPorEnviar = nil
		s.aplicar(t, n)
	case errors.Is(err, puertos.ErrEstadoNoPermite), errors.Is(err, puertos.ErrRechazoDelPedido):
		// El traslado salió mientras tanto: nos quedamos con su estado actual.
		t.CancelacionPorEnviar = nil
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("Rumbo no aceptó la cancelación de la transferencia %s: %v", t.ID, err))
		actual, err := s.Transporte.Consultar(t)
		if err != nil {
			s.guardar(t)
			return
		}
		s.aplicar(t, actual)
	default:
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("no se pudo mandar la cancelación de la transferencia %s, se reintenta: %v", t.ID, err))
	}
}

// aplicar actualiza la transferencia con lo que informó Rumbo, la guarda y, si cambió de estado, lo publica.
func (s *Servicio) aplicar(t *dominio.Transferencia, n dominio.Novedad) {
	if s.trasladoSuelto(t, n) {
		return
	}
	anterior := t.Estado
	if !t.Aplicar(n, s.Ahora()) {
		s.guardar(t)
		return
	}
	s.Registro.Escribir(t.TrackID, "info", fmt.Sprintf("transferencia %s: %s -> %s", t.ID, anterior, t.Estado))
	s.guardarYPublicar(t)
}

// trasladoSuelto se fija si Rumbo tiene programado el traslado de una transferencia que de nuestro lado
// ya terminó. Pasa si el usuario la canceló mientras no sabíamos si Rumbo había recibido el pedido.
// Lo cancelamos en Rumbo, así el lugar de la franja queda libre para los demás grupos.
func (s *Servicio) trasladoSuelto(t *dominio.Transferencia, n dominio.Novedad) bool {
	if !t.Estado.LiberaLaReserva() || n.Estado != dominio.Programada {
		return false
	}
	if t.CancelacionPorEnviar == nil {
		motivo := fmt.Sprintf("la transferencia ya estaba %s en Stock Volcamelli", t.Estado)
		t.IDRumbo = n.IDRumbo
		t.CancelacionPorEnviar = &motivo
		s.guardar(t)
		s.Registro.Escribir(t.TrackID, "warn", fmt.Sprintf("Rumbo tiene programado el traslado %s de la transferencia %s, que ya está %s: lo cancelamos", n.IDRumbo, t.ID, t.Estado))
	}
	return true
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
