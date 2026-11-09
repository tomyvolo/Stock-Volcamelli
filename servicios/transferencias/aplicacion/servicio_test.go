package aplicacion

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// escenario arma el Servicio con los puertos falsos. El origen (sucursal 1) tiene 10 disponibles del producto 1.
type escenario struct {
	servicio   *Servicio
	repo       *repoFalso
	inventario *inventarioFalso
	transporte *transporteFalso
	eventos    *eventosFalsos
	ahora      time.Time
}

func nuevoEscenario() *escenario {
	e := &escenario{
		repo:       &repoFalso{transferencias: map[string]*dominio.Transferencia{}},
		inventario: &inventarioFalso{disponible: map[string]int{"1-1": 10}, reservas: map[string]int{}},
		transporte: &transporteFalso{},
		eventos:    &eventosFalsos{},
		ahora:      time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
	}
	ids := 0
	e.servicio = &Servicio{
		Repo: e.repo, Inventario: e.inventario, Sucursales: sucursalesFalsas{}, Transporte: e.transporte,
		Eventos: e.eventos, Registro: registroFalso{},
		Ahora:   func() time.Time { return e.ahora },
		NuevoID: func() string { ids++; return fmt.Sprintf("id-%d", ids) },
	}
	return e
}

func sucursal(id uint) *uint { return &id }

var vendedor1 = Usuario{ID: 3, Rol: "VENDEDOR", SucursalID: sucursal(1)}

func pedido(cantidad int) Pedido {
	return Pedido{ProductoID: 1, Cantidad: cantidad, OrigenID: 1, DestinoID: 2}
}

// novedad arma lo que informaría Rumbo, un minuto después de la anterior.
func (e *escenario) novedad(t *dominio.Transferencia, estado dominio.Estado) dominio.Novedad {
	e.ahora = e.ahora.Add(time.Minute)
	return dominio.Novedad{TransferenciaID: t.ID, IDRumbo: "sol-1", Estado: estado, ActualizadoEn: e.ahora}
}

// creadaYEnviada crea la transferencia de CA-01 y la manda a Rumbo.
func (e *escenario) creadaYEnviada(t *testing.T, p Pedido) *dominio.Transferencia {
	transferencia, err := e.servicio.Crear(p, vendedor1, "track-1")
	if err != nil {
		t.Fatalf("No se pudo crear la transferencia: %v", err)
	}
	e.transporte.respSolicitar = e.novedad(transferencia, dominio.Reservada)
	e.servicio.EnviarPendientes()
	return transferencia
}

// CA-01: la transferencia queda RESERVADA y el origen tiene 6 disponibles.
func TestCrear_Reservada(t *testing.T) {
	e := nuevoEscenario()

	transferencia, err := e.servicio.Crear(pedido(4), vendedor1, "track-1")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if transferencia.Estado != dominio.Reservada || e.inventario.disponible["1-1"] != 6 {
		t.Errorf("Se esperaba RESERVADA con 6 disponibles, pero quedó %s con %d", transferencia.Estado, e.inventario.disponible["1-1"])
	}
	if transferencia.NombreProducto != "Harina 000" || transferencia.Origen.Nombre != "Sucursal 1" {
		t.Errorf("Faltan los datos para Rumbo: %+v", transferencia)
	}
	if !slices.Equal(e.eventos.tipos(transferencia.ID), []string{puertos.TransferenciaActualizada}) {
		t.Errorf("Se esperaba solo TransferenciaActualizada, pero se publicó: %v", e.eventos.tipos(transferencia.ID))
	}
	// Tiempo Real necesita el origen y el destino para que cada pantalla sepa si el aviso le toca.
	if evento := e.eventos.publicados[0]; evento.OrigenID != transferencia.OrigenID || evento.DestinoID != transferencia.DestinoID {
		t.Errorf("El evento tenía que llevar el origen %d y el destino %d, pero llevó %+v", transferencia.OrigenID, transferencia.DestinoID, evento)
	}
}

// CA-02: sin stock suficiente queda SIN_STOCK y no se reserva nada.
func TestCrear_SinStock(t *testing.T) {
	e := nuevoEscenario()

	transferencia, err := e.servicio.Crear(pedido(11), vendedor1, "track-1")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if transferencia.Estado != dominio.SinStock || e.inventario.disponible["1-1"] != 10 {
		t.Errorf("Se esperaba SIN_STOCK sin tocar el stock, pero quedó %s con %d", transferencia.Estado, e.inventario.disponible["1-1"])
	}
}

// CA-03, CA-04 y CA-05: misma sucursal, sucursal inexistente y hora pasada se rechazan sin reservar.
func TestCrear_PedidosInvalidos(t *testing.T) {
	ayer := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre   string
		cambiar  func(p *Pedido)
		esperado error
	}{
		{"misma sucursal", func(p *Pedido) { p.DestinoID = 1 }, dominio.ErrMismaSucursal},
		{"sucursal inexistente", func(p *Pedido) { p.DestinoID = 9 }, ErrSucursalInexistente},
		{"hora pasada", func(p *Pedido) { p.HoraLlegada = &ayer }, dominio.ErrHoraPasada},
		{"producto inexistente", func(p *Pedido) { p.ProductoID = 99 }, ErrProductoInexistente},
	}
	for _, caso := range casos {
		e := nuevoEscenario()
		p := pedido(4)
		caso.cambiar(&p)

		_, err := e.servicio.Crear(p, vendedor1, "track-1")

		if !errors.Is(err, caso.esperado) {
			t.Errorf("%s: se esperaba %v, pero se obtuvo %v", caso.nombre, caso.esperado, err)
		}
		if len(e.inventario.reservas) != 0 || len(e.repo.transferencias) != 0 {
			t.Errorf("%s: no tenía que reservar ni guardar nada", caso.nombre)
		}
	}
}

// CA-20: un vendedor de la sucursal 3 no puede transferir de la 1 a la 2.
func TestCrear_SinPermiso(t *testing.T) {
	e := nuevoEscenario()
	otro := Usuario{ID: 5, Rol: "VENDEDOR", SucursalID: sucursal(3)}

	_, err := e.servicio.Crear(pedido(4), otro, "track-1")

	if !errors.Is(err, ErrSinPermiso) {
		t.Errorf("Se esperaba ErrSinPermiso, pero se obtuvo: %v", err)
	}
}

// CA-14: si Inventario no responde, queda FALLIDA y después se libera la reserva por si se llegó a hacer.
func TestCrear_InventarioNoResponde(t *testing.T) {
	e := nuevoEscenario()
	e.inventario.errReserva = puertos.ErrNoDisponible

	transferencia, err := e.servicio.Crear(pedido(4), vendedor1, "track-1")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if transferencia.Estado != dominio.Fallida || !transferencia.LiberacionPorEnviar {
		t.Fatalf("Se esperaba FALLIDA con la liberación pendiente, pero quedó: %+v", transferencia)
	}

	e.servicio.EnviarPendientes()

	if !slices.Contains(e.inventario.liberadas, transferencia.ID) || transferencia.LiberacionPorEnviar {
		t.Errorf("Se esperaba que se pidiera liberar la reserva: %v", e.inventario.liberadas)
	}
}

func TestEnviarPendientes_MandaARumbo(t *testing.T) {
	e := nuevoEscenario()

	transferencia := e.creadaYEnviada(t, pedido(4))

	if transferencia.IDRumbo != "sol-1" || transferencia.Estado != dominio.Reservada {
		t.Errorf("Se esperaba RESERVADA con el id de Rumbo, pero quedó: %+v", transferencia)
	}
}

// Si Rumbo rechaza el pedido por datos inválidos (422), queda RECHAZADA y se libera la reserva.
func TestEnviarPendientes_RumboRechazaElPedido(t *testing.T) {
	e := nuevoEscenario()
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")
	e.transporte.respSolicitar = dominio.Novedad{Motivo: "Lo solicitado excede la capacidad de la flota"}
	e.transporte.errSolicitar = puertos.ErrRechazoDelPedido

	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Rechazada || !slices.Contains(e.eventos.tipos(transferencia.ID), puertos.TrasladoCancelado) {
		t.Errorf("Se esperaba RECHAZADA con TrasladoCancelado, pero quedó %s: %v", transferencia.Estado, e.eventos.tipos(transferencia.ID))
	}
}

// Si pasan 24 horas sin poder mandarle el pedido a Rumbo, queda RECHAZADA.
func TestEnviarPendientes_VenceA24Horas(t *testing.T) {
	e := nuevoEscenario()
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")
	e.transporte.errSolicitar = puertos.ErrNoDisponible

	e.servicio.EnviarPendientes()
	if transferencia.Estado != dominio.Reservada {
		t.Fatalf("Antes de las 24 horas tenía que seguir RESERVADA, pero quedó %s", transferencia.Estado)
	}
	e.ahora = e.ahora.Add(PlazoParaEnviar)
	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Rechazada {
		t.Errorf("Se esperaba RECHAZADA, pero quedó %s", transferencia.Estado)
	}
}

// CA-06: sin hora pedida, el horario propuesto se acepta solo y sigue RESERVADA hasta que Rumbo la programa.
func TestCambios_PropuestaSinHoraPedida(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	propuesta := e.novedad(transferencia, dominio.Reservada)
	propuesta.Contrapropuesta = true
	e.transporte.cambios = []dominio.Novedad{propuesta}

	e.servicio.ConsultarCambios()

	if transferencia.Estado != dominio.Reservada || transferencia.RespuestaPorEnviar == nil || !*transferencia.RespuestaPorEnviar {
		t.Fatalf("Se esperaba RESERVADA con la aceptación pendiente, pero quedó: %+v", transferencia)
	}

	e.transporte.respResponder = e.novedad(transferencia, dominio.Programada)
	e.servicio.EnviarPendientes()

	if e.transporte.respondido == nil || !*e.transporte.respondido || transferencia.Estado != dominio.Programada {
		t.Errorf("Se esperaba aceptar solo y quedar PROGRAMADA, pero quedó %s", transferencia.Estado)
	}
}

// CA-07: con hora pedida, el horario propuesto espera la respuesta del usuario.
func TestCambios_PropuestaConHoraPedida(t *testing.T) {
	e := nuevoEscenario()
	manana := e.ahora.Add(24 * time.Hour)
	p := pedido(4)
	p.HoraLlegada = &manana
	transferencia := e.creadaYEnviada(t, p)
	propuesta := e.novedad(transferencia, dominio.Reservada)
	propuesta.Contrapropuesta = true
	otraHora, vence := manana.Add(3*time.Hour), e.ahora.Add(24*time.Hour)
	propuesta.HoraPropuesta, propuesta.VencePropuesta = &otraHora, &vence
	e.transporte.cambios = []dominio.Novedad{propuesta}

	e.servicio.ConsultarCambios()

	if transferencia.Estado != dominio.EsperandoConfirmacion || transferencia.RespuestaPorEnviar != nil {
		t.Fatalf("Se esperaba ESPERANDO_CONFIRMACION sin responder, pero quedó: %+v", transferencia)
	}

	if _, err := e.servicio.Responder(transferencia.ID, vendedor1, true); err != nil {
		t.Fatalf("No se pudo responder: %v", err)
	}
	e.transporte.respResponder = e.novedad(transferencia, dominio.Programada)
	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Programada {
		t.Errorf("Se esperaba PROGRAMADA después de aceptar, pero quedó %s", transferencia.Estado)
	}
}

// CA-08: cuando el traslado llega queda COMPLETADA y se avisa a Inventario para que mueva el stock.
func TestCambios_Completada(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	e.transporte.cambios = []dominio.Novedad{e.novedad(transferencia, dominio.EnCurso), e.novedad(transferencia, dominio.Completada)}

	e.servicio.ConsultarCambios()

	if transferencia.Estado != dominio.Completada || !slices.Contains(e.eventos.tipos(transferencia.ID), puertos.TrasladoCompletado) {
		t.Errorf("Se esperaba COMPLETADA con TrasladoCompletado, pero quedó %s: %v", transferencia.Estado, e.eventos.tipos(transferencia.ID))
	}
}

// CA-09: si Rumbo la rechaza queda RECHAZADA con su motivo y se avisa a Inventario para que libere.
func TestCambios_Rechazada(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	rechazo := e.novedad(transferencia, dominio.Rechazada)
	rechazo.Motivo = "No hay vehículos con capacidad disponible para esa fecha."
	e.transporte.cambios = []dominio.Novedad{rechazo}

	e.servicio.ConsultarCambios()

	if transferencia.Estado != dominio.Rechazada || transferencia.Motivo != rechazo.Motivo {
		t.Errorf("Se esperaba RECHAZADA con el motivo de Rumbo, pero quedó: %+v", transferencia)
	}
	if !slices.Contains(e.eventos.tipos(transferencia.ID), puertos.TrasladoCancelado) {
		t.Errorf("Se esperaba TrasladoCancelado, pero se publicó: %v", e.eventos.tipos(transferencia.ID))
	}
}

// Un dato de Rumbo más viejo que el último que aplicamos no hace volver atrás a la transferencia.
func TestCambios_NoVuelveAtras(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	viejo := e.novedad(transferencia, dominio.Programada)
	nuevo := e.novedad(transferencia, dominio.EnCurso)
	e.transporte.cambios = []dominio.Novedad{nuevo, viejo}

	e.servicio.ConsultarCambios()

	if transferencia.Estado != dominio.EnCurso {
		t.Errorf("Se esperaba EN_CURSO, pero volvió a %s", transferencia.Estado)
	}
}

// Si Rumbo todavía no tenía el pedido, la cancelación es inmediata y se libera la reserva.
func TestCancelar_AntesDeMandarla(t *testing.T) {
	e := nuevoEscenario()
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")

	_, err := e.servicio.Cancelar(transferencia.ID, vendedor1, "Ya no hace falta")

	if err != nil || transferencia.Estado != dominio.Cancelada {
		t.Fatalf("Se esperaba CANCELADA, pero quedó %s (%v)", transferencia.Estado, err)
	}
	if !slices.Contains(e.eventos.tipos(transferencia.ID), puertos.TrasladoCancelado) {
		t.Errorf("Se esperaba TrasladoCancelado, pero se publicó: %v", e.eventos.tipos(transferencia.ID))
	}
}

// CA-10: una transferencia PROGRAMADA se cancela a través de Rumbo y se libera la reserva.
func TestCancelar_Programada(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	e.transporte.cambios = []dominio.Novedad{e.novedad(transferencia, dominio.Programada)}
	e.servicio.ConsultarCambios()

	if _, err := e.servicio.Cancelar(transferencia.ID, vendedor1, "Ya no hace falta"); err != nil {
		t.Fatalf("No se pudo cancelar: %v", err)
	}
	e.transporte.respCancelar = e.novedad(transferencia, dominio.Cancelada)
	e.servicio.EnviarPendientes()

	if e.transporte.motivoCancelar != "Ya no hace falta" || transferencia.Estado != dominio.Cancelada {
		t.Errorf("Se esperaba CANCELADA con el motivo enviado a Rumbo, pero quedó %s", transferencia.Estado)
	}
	if !slices.Contains(e.eventos.tipos(transferencia.ID), puertos.TrasladoCancelado) {
		t.Errorf("Se esperaba TrasladoCancelado, pero se publicó: %v", e.eventos.tipos(transferencia.ID))
	}
}

// CA-11: un traslado EN_CURSO no se puede cancelar.
func TestCancelar_EnCurso(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	e.transporte.cambios = []dominio.Novedad{e.novedad(transferencia, dominio.EnCurso)}
	e.servicio.ConsultarCambios()

	_, err := e.servicio.Cancelar(transferencia.ID, vendedor1, "Ya no hace falta")

	if !errors.Is(err, dominio.ErrNoCancelable) || transferencia.Estado != dominio.EnCurso {
		t.Errorf("Se esperaba ErrNoCancelable y seguir EN_CURSO, pero se obtuvo %v y %s", err, transferencia.Estado)
	}
}

// Con menos de 24 horas, Rumbo deja la cancelación pendiente; si no la aprueba, vuelve a PROGRAMADA.
func TestCancelar_PendienteYRechazada(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	e.transporte.cambios = []dominio.Novedad{e.novedad(transferencia, dominio.Programada)}
	e.servicio.ConsultarCambios()
	e.servicio.Cancelar(transferencia.ID, vendedor1, "Ya no hace falta")

	pendiente := e.novedad(transferencia, dominio.Programada)
	pendiente.CancelacionPendiente = true
	e.transporte.respCancelar = pendiente
	e.servicio.EnviarPendientes()
	if transferencia.Estado != dominio.CancelacionPendiente {
		t.Fatalf("Se esperaba CANCELACION_PENDIENTE, pero quedó %s", transferencia.Estado)
	}

	noAprobada := e.novedad(transferencia, dominio.Programada)
	noAprobada.CancelacionRechazada = true
	e.transporte.cambios = []dominio.Novedad{noAprobada}
	e.servicio.ConsultarCambios()

	if transferencia.Estado != dominio.Programada || !transferencia.CancelacionRechazada {
		t.Errorf("Se esperaba PROGRAMADA con la cancelación rechazada, pero quedó: %+v", transferencia)
	}
}
