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

// escenario arma el Servicio con los puertos falsos. El origen (sucursal 1) tiene 10 disponibles del producto 1,
// y Rumbo tiene lugar hoy de 14 a 16 y de 16 a 18.
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
		transporte: &transporteFalso{franjas: map[string][]dominio.Franja{"2026-11-02": {franja(2, 14), franja(2, 16)}}},
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

// franja arma la franja de 2 horas que empieza el día dia de noviembre a la hora indicada.
func franja(dia, hora int) dominio.Franja {
	inicio := time.Date(2026, 11, dia, hora, 0, 0, 0, time.UTC)
	return dominio.Franja{Inicio: inicio, Fin: inicio.Add(2 * time.Hour)}
}

var vendedor1 = Usuario{ID: 3, Rol: "VENDEDOR", SucursalID: sucursal(1)}

func pedido(cantidad int) Pedido {
	return Pedido{ProductoID: 1, Cantidad: cantidad, OrigenID: 1, DestinoID: 2}
}

// novedad arma lo que informaría Rumbo, un minuto después de la anterior.
func (e *escenario) novedad(t *dominio.Transferencia, estado dominio.Estado) dominio.Novedad {
	e.ahora = e.ahora.Add(time.Minute)
	return dominio.Novedad{TransferenciaID: t.ID, IDRumbo: "sol-1", Estado: estado, ActualizadoEn: e.ahora}
}

// creadaYEnviada crea la transferencia de CA-01 y la reserva en Rumbo.
func (e *escenario) creadaYEnviada(t *testing.T, p Pedido) *dominio.Transferencia {
	transferencia, err := e.servicio.Crear(p, vendedor1, "track-1")
	if err != nil {
		t.Fatalf("No se pudo crear la transferencia: %v", err)
	}
	e.transporte.respReservar = e.novedad(transferencia, dominio.Programada)
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

// CA-03, CA-04 y CA-05: misma sucursal, sucursal inexistente, hora pasada y hora que Rumbo no puede
// reservar se rechazan sin reservar.
func TestCrear_PedidosInvalidos(t *testing.T) {
	ayer := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	enDosMeses := time.Date(2027, 1, 2, 10, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre   string
		cambiar  func(p *Pedido)
		esperado error
	}{
		{"misma sucursal", func(p *Pedido) { p.DestinoID = 1 }, dominio.ErrMismaSucursal},
		{"sucursal inexistente", func(p *Pedido) { p.DestinoID = 9 }, ErrSucursalInexistente},
		{"hora pasada", func(p *Pedido) { p.HoraLlegada = &ayer }, dominio.ErrHoraPasada},
		{"hora a más de 30 días", func(p *Pedido) { p.HoraLlegada = &enDosMeses }, dominio.ErrHoraLejana},
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

// CA-06: sin hora pedida, se reserva la primera franja con lugar y queda PROGRAMADA.
func TestEnviarPendientes_ReservaLaPrimeraFranja(t *testing.T) {
	e := nuevoEscenario()

	transferencia := e.creadaYEnviada(t, pedido(4))

	if transferencia.Estado != dominio.Programada || transferencia.IDRumbo != "sol-1" {
		t.Fatalf("Se esperaba PROGRAMADA con el id de Rumbo, pero quedó: %+v", transferencia)
	}
	if *transferencia.Franja != franja(2, 14) || e.inventario.disponible["1-1"] != 6 {
		t.Errorf("Se esperaba la franja de 14 a 16 con las 4 unidades reservadas, pero quedó %+v con %d", transferencia.Franja, e.inventario.disponible["1-1"])
	}
}

// Con hora pedida, se reserva la franja que la contiene.
func TestEnviarPendientes_FranjaDeLaHoraPedida(t *testing.T) {
	e := nuevoEscenario()
	alas17 := time.Date(2026, 11, 2, 17, 0, 0, 0, time.UTC)
	p := pedido(4)
	p.HoraLlegada = &alas17

	transferencia := e.creadaYEnviada(t, p)

	if transferencia.Estado != dominio.Programada || *transferencia.Franja != franja(2, 16) {
		t.Errorf("Se esperaba PROGRAMADA de 16 a 18, pero quedó %s con %+v", transferencia.Estado, transferencia.Franja)
	}
}

// CA-07: si la franja de la hora pedida se llena antes de reservarla, se reserva la siguiente con lugar.
func TestEnviarPendientes_FranjaLlena(t *testing.T) {
	e := nuevoEscenario()
	alas15 := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	p := pedido(4)
	p.HoraLlegada = &alas15
	transferencia, _ := e.servicio.Crear(p, vendedor1, "track-1")
	e.transporte.errReservar = []error{puertos.ErrSinLugar}

	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Reservada || transferencia.Franja != nil {
		t.Fatalf("Se esperaba RESERVADA y sin franja para buscar otra, pero quedó: %+v", transferencia)
	}

	// Otro grupo tomó el último lugar: la disponibilidad ya no muestra la franja de 14 a 16.
	e.transporte.franjas["2026-11-02"] = []dominio.Franja{franja(2, 16)}
	e.transporte.respReservar = e.novedad(transferencia, dominio.Programada)
	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Programada || *transferencia.Franja != franja(2, 16) {
		t.Errorf("Se esperaba PROGRAMADA de 16 a 18, pero quedó %s con %+v", transferencia.Estado, transferencia.Franja)
	}
}

// Si el día no tiene lugar, se busca en los días siguientes.
func TestEnviarPendientes_BuscaEnLosDiasSiguientes(t *testing.T) {
	e := nuevoEscenario()
	e.transporte.franjas = map[string][]dominio.Franja{"2026-11-04": {franja(4, 8)}}

	transferencia := e.creadaYEnviada(t, pedido(4))

	if transferencia.Estado != dominio.Programada || *transferencia.Franja != franja(4, 8) {
		t.Errorf("Se esperaba PROGRAMADA el 4/11 de 8 a 10, pero quedó %s con %+v", transferencia.Estado, transferencia.Franja)
	}
}

// CA-09: si Rumbo no tiene lugar en 30 días, queda RECHAZADA y se libera la reserva.
func TestEnviarPendientes_SinLugarEn30Dias(t *testing.T) {
	e := nuevoEscenario()
	e.transporte.franjas = nil
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")

	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Rechazada || !slices.Contains(e.eventos.tipos(transferencia.ID), puertos.TrasladoCancelado) {
		t.Errorf("Se esperaba RECHAZADA con TrasladoCancelado, pero quedó %s: %v", transferencia.Estado, e.eventos.tipos(transferencia.ID))
	}
	if e.transporte.diasConsultados != dominio.DiasReservables+1 || len(e.transporte.reservadas) != 0 {
		t.Errorf("Se esperaba consultar %d días sin reservar nada, pero consultó %d y reservó %v",
			dominio.DiasReservables+1, e.transporte.diasConsultados, e.transporte.reservadas)
	}
}

// Si Rumbo no responde, el reintento va a la misma franja (y por eso con la misma clave): no reserva dos veces.
func TestEnviarPendientes_ReintentaLaMismaFranja(t *testing.T) {
	e := nuevoEscenario()
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")
	e.transporte.errReservar = []error{puertos.ErrNoDisponible}

	e.servicio.EnviarPendientes()
	// Mientras tanto la disponibilidad cambió, pero el reintento no la vuelve a consultar.
	e.transporte.franjas["2026-11-02"] = []dominio.Franja{franja(2, 16)}
	e.transporte.respReservar = e.novedad(transferencia, dominio.Programada)
	e.servicio.EnviarPendientes()

	if len(e.transporte.reservadas) != 2 || e.transporte.reservadas[0] != e.transporte.reservadas[1] {
		t.Errorf("Se esperaba reintentar la misma franja, pero se pidió: %v", e.transporte.reservadas)
	}
	if transferencia.Estado != dominio.Programada {
		t.Errorf("Se esperaba PROGRAMADA, pero quedó %s", transferencia.Estado)
	}
}

// Si Rumbo rechaza el pedido por datos inválidos (422), queda RECHAZADA y se libera la reserva.
func TestEnviarPendientes_RumboRechazaElPedido(t *testing.T) {
	e := nuevoEscenario()
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")
	e.transporte.errReservar = []error{puertos.ErrRechazoDelPedido}
	e.transporte.motivoRechazo = "Rumbo rechazó el pedido: El peso supera el cupo total de una franja"

	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Rechazada || transferencia.Motivo != e.transporte.motivoRechazo {
		t.Errorf("Se esperaba RECHAZADA con el motivo de Rumbo, pero quedó: %+v", transferencia)
	}
	if !slices.Contains(e.eventos.tipos(transferencia.ID), puertos.TrasladoCancelado) {
		t.Errorf("Se esperaba TrasladoCancelado, pero se publicó: %v", e.eventos.tipos(transferencia.ID))
	}
}

// Si pasan 24 horas sin poder reservar en Rumbo, queda RECHAZADA.
func TestEnviarPendientes_VenceA24Horas(t *testing.T) {
	e := nuevoEscenario()
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")
	e.transporte.errSiempre = puertos.ErrNoDisponible

	e.servicio.EnviarPendientes()
	if transferencia.Estado != dominio.Reservada {
		t.Fatalf("Antes de las 24 horas tenía que seguir RESERVADA, pero quedó %s", transferencia.Estado)
	}
	e.ahora = e.ahora.Add(PlazoParaReservar)
	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Rechazada {
		t.Errorf("Se esperaba RECHAZADA, pero quedó %s", transferencia.Estado)
	}
}

// Sin Rumbo configurado, la transferencia se crea y queda RESERVADA (así corren las pruebas de carga).
func TestEnviarPendientes_SinRumbo(t *testing.T) {
	e := nuevoEscenario()
	e.servicio.Transporte = nil
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")

	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.Reservada || transferencia.Franja != nil {
		t.Errorf("Se esperaba RESERVADA sin franja, pero quedó: %+v", transferencia)
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

// CA-12: si Rumbo informa que el traslado no se hizo, queda NO_REALIZADA con su motivo y se libera la reserva.
func TestCambios_NoRealizada(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	noRealizado := e.novedad(transferencia, dominio.NoRealizada)
	noRealizado.Motivo = "La sucursal de origen estaba cerrada."
	e.transporte.cambios = []dominio.Novedad{noRealizado}

	e.servicio.ConsultarCambios()

	if transferencia.Estado != dominio.NoRealizada || transferencia.Motivo != noRealizado.Motivo {
		t.Errorf("Se esperaba NO_REALIZADA con el motivo de Rumbo, pero quedó: %+v", transferencia)
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

// Si el usuario cancela mientras no sabíamos si Rumbo había reservado, y después aparece el traslado
// programado, lo cancelamos en Rumbo para liberar el lugar de la franja.
func TestCambios_TrasladoSuelto(t *testing.T) {
	e := nuevoEscenario()
	transferencia, _ := e.servicio.Crear(pedido(4), vendedor1, "track-1")
	e.transporte.errReservar = []error{puertos.ErrNoDisponible}
	e.servicio.EnviarPendientes()
	e.servicio.Cancelar(transferencia.ID, vendedor1, "Ya no hace falta")
	eventos := len(e.eventos.tipos(transferencia.ID))

	suelto := e.novedad(transferencia, dominio.Programada)
	suelto.IDRumbo = "sol-9"
	e.transporte.cambios = []dominio.Novedad{suelto}
	e.servicio.ConsultarCambios()
	e.transporte.respCancelar = e.novedad(transferencia, dominio.Cancelada)
	e.servicio.EnviarPendientes()

	if e.transporte.canceladoEn != "sol-9" || transferencia.CancelacionPorEnviar != nil {
		t.Errorf("Se esperaba cancelar en Rumbo el traslado sol-9, pero se canceló %q", e.transporte.canceladoEn)
	}
	if transferencia.Estado != dominio.Cancelada || len(e.eventos.tipos(transferencia.ID)) != eventos {
		t.Errorf("Se esperaba que siga CANCELADA sin eventos nuevos, pero quedó %s: %v", transferencia.Estado, e.eventos.tipos(transferencia.ID))
	}
}

// Si Rumbo todavía no reservó el traslado, la cancelación es inmediata y se libera la reserva.
func TestCancelar_AntesDeReservar(t *testing.T) {
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

	if _, err := e.servicio.Cancelar(transferencia.ID, vendedor1, "Ya no hace falta"); err != nil {
		t.Fatalf("No se pudo cancelar: %v", err)
	}
	if transferencia.Estado != dominio.Programada || transferencia.CancelacionPorEnviar == nil {
		t.Fatalf("Hasta que Rumbo responda tenía que seguir PROGRAMADA con la cancelación pedida, pero quedó: %+v", transferencia)
	}
	cancelado := e.novedad(transferencia, dominio.Cancelada)
	cancelado.Motivo = "Ya no hace falta"
	e.transporte.respCancelar = cancelado
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

// Si el camión salió entre que el usuario canceló y se lo mandamos a Rumbo, queda EN_CURSO.
func TestCancelar_SalioMientrasTanto(t *testing.T) {
	e := nuevoEscenario()
	transferencia := e.creadaYEnviada(t, pedido(4))
	e.servicio.Cancelar(transferencia.ID, vendedor1, "Ya no hace falta")
	e.transporte.errCancelar = puertos.ErrEstadoNoPermite
	e.transporte.respConsultar = e.novedad(transferencia, dominio.EnCurso)

	e.servicio.EnviarPendientes()

	if transferencia.Estado != dominio.EnCurso || transferencia.CancelacionPorEnviar != nil {
		t.Errorf("Se esperaba EN_CURSO sin la cancelación pendiente, pero quedó: %+v", transferencia)
	}
}
