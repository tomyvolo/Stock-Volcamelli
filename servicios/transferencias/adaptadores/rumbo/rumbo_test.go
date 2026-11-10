package rumbo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Estos tests levantan, dentro de cada test, un servidor que responde como el contrato de Rumbo
// (docs/contracts/rumbo/traslados.yaml). Así probamos cada caso sin depender de que Rumbo esté andando.

type registroVacio struct{}

func (registroVacio) Escribir(trackID, nivel, mensaje string) {}

// rumboDePrueba responde el token y le pasa el resto de los pedidos a responder.
type rumboDePrueba struct {
	responder func(w http.ResponseWriter, r *http.Request)
	tokens    int
	pedidos   []*http.Request
	cuerpos   []map[string]any
}

func (p *rumboDePrueba) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/auth/token" {
		p.tokens++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":"token-%d","token_type":"Bearer","expira_en_segundos":3600}`, p.tokens)
		return
	}
	var cuerpo map[string]any
	json.NewDecoder(r.Body).Decode(&cuerpo)
	p.pedidos = append(p.pedidos, r)
	p.cuerpos = append(p.cuerpos, cuerpo)
	p.responder(w, r)
}

func nuevoCliente(t *testing.T, responder func(w http.ResponseWriter, r *http.Request)) (*Cliente, *rumboDePrueba) {
	prueba := &rumboDePrueba{responder: responder}
	servidor := httptest.NewServer(prueba)
	t.Cleanup(servidor.Close)
	return &Cliente{BaseURL: servidor.URL, ClientID: "id", ClientSecret: "secreto", HTTP: servidor.Client(), Registro: registroVacio{}}, prueba
}

func responderJSON(w http.ResponseWriter, codigo int, cuerpo string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(codigo)
	fmt.Fprint(w, cuerpo)
}

func problemaJSON(codigo int, codigoRumbo string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, codigo, fmt.Sprintf(`{"type":"x","title":"Error","status":%d,"codigo":%q,"detail":"detalle de Rumbo"}`, codigo, codigoRumbo))
	}
}

const trasladoProgramado = `{"id":"sol-7f3a9c","referencia_externa":"trf-1","estado":"PROGRAMADO",
	"franja":{"inicio":"2026-11-20T10:00:00-03:00","fin":"2026-11-20T12:00:00-03:00"},
	"creado_en":"2026-11-19T08:30:00-03:00","actualizado_en":"2026-11-19T08:30:00-03:00"}`

func transferenciaConFranja() *dominio.Transferencia {
	inicio := time.Date(2026, 11, 20, 13, 0, 0, 0, time.UTC) // 10:00 en Argentina
	return &dominio.Transferencia{
		ID: "trf-1", NombreProducto: "Harina 000", Cantidad: 4, Observaciones: "Portón de carga",
		Origen:  dominio.Ubicacion{Nombre: "Sucursal Centro", Direccion: "Av. Colón 1234"},
		Destino: dominio.Ubicacion{Nombre: "Sucursal Cerro", Direccion: "Av. Rafael Núñez 4500"},
		Franja:  &dominio.Franja{Inicio: inicio, Fin: inicio.Add(2 * time.Hour)},
	}
}

func TestFranjas_SoloLasQueTienenLugar(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, `{"fecha":"2026-11-20","franjas":[
			{"inicio":"2026-11-20T12:00:00-03:00","fin":"2026-11-20T14:00:00-03:00","disponible":true,"traslados_disponibles":3,"kg_disponibles":600},
			{"inicio":"2026-11-20T10:00:00-03:00","fin":"2026-11-20T12:00:00-03:00","disponible":false,"traslados_disponibles":0,"kg_disponibles":0},
			{"inicio":"2026-11-20T08:00:00-03:00","fin":"2026-11-20T10:00:00-03:00","disponible":true,"traslados_disponibles":4,"kg_disponibles":2000}]}`)
	})

	franjas, err := cliente.Franjas(time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC))

	if err != nil || len(franjas) != 2 {
		t.Fatalf("Se esperaban las 2 franjas con lugar, pero se obtuvo %v (%v)", franjas, err)
	}
	if franjas[0].Inicio.Hour() != 8 || franjas[1].Inicio.Hour() != 12 {
		t.Errorf("Se esperaban en orden la de 8 y la de 12, pero se obtuvo %v", franjas)
	}
	if fecha := prueba.pedidos[0].URL.Query().Get("fecha"); fecha != "2026-11-20" {
		t.Errorf("Se esperaba consultar el 2026-11-20, pero se consultó %q", fecha)
	}
}

// La fecha se manda en hora de Argentina: las 2 de la mañana UTC del 21 todavía son el 20 en Argentina.
func TestFranjas_FechaEnHoraDeArgentina(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, `{"fecha":"2026-11-20","franjas":[]}`)
	})

	cliente.Franjas(time.Date(2026, 11, 21, 2, 0, 0, 0, time.UTC))

	if fecha := prueba.pedidos[0].URL.Query().Get("fecha"); fecha != "2026-11-20" {
		t.Errorf("Se esperaba consultar el 2026-11-20, pero se consultó %q", fecha)
	}
}

// Un día fuera de lo que se puede reservar no tiene franjas.
func TestFranjas_DiaFueraDeRango(t *testing.T) {
	cliente, _ := nuevoCliente(t, problemaJSON(http.StatusUnprocessableEntity, "DATOS_INVALIDOS"))

	franjas, err := cliente.Franjas(time.Now().AddDate(0, 0, 40))

	if err != nil || len(franjas) != 0 {
		t.Errorf("Se esperaba ninguna franja y sin error, pero se obtuvo %v (%v)", franjas, err)
	}
}

func TestReservar_Programado(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusCreated, trasladoProgramado)
	})
	transferencia := transferenciaConFranja()

	n, err := cliente.Reservar(transferencia)

	if err != nil || n.Estado != dominio.Programada || n.IDRumbo != "sol-7f3a9c" {
		t.Fatalf("Se esperaba PROGRAMADA con el id de Rumbo, pero se obtuvo %+v (%v)", n, err)
	}
	pedido, cuerpo := prueba.pedidos[0], prueba.cuerpos[0]
	if pedido.Method != http.MethodPost || pedido.URL.Path != "/traslados" || pedido.Header.Get("Authorization") != "Bearer token-1" {
		t.Errorf("Se esperaba POST /traslados con el token, pero se mandó %s %s", pedido.Method, pedido.URL.Path)
	}
	if cuerpo["franja_inicio"] != "2026-11-20T10:00:00-03:00" || cuerpo["referencia_externa"] != "trf-1" {
		t.Errorf("Se esperaba la franja en hora de Argentina y nuestra referencia, pero se mandó %v", cuerpo)
	}
	if cosa, _ := cuerpo["cosa"].(map[string]any); cosa["descripcion"] != "Harina 000" || cosa["cantidad"] != 4.0 {
		t.Errorf("Se esperaba la cosa con el producto y la cantidad, pero se mandó %v", cuerpo["cosa"])
	}
}

// La clave es la misma para la misma franja (reintento) y cambia con otra franja (otra intención).
func TestReservar_ClaveDeIdempotencia(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusCreated, trasladoProgramado)
	})
	transferencia := transferenciaConFranja()

	cliente.Reservar(transferencia)
	cliente.Reservar(transferencia)
	otra := transferencia.Franja.Inicio.Add(2 * time.Hour)
	transferencia.Franja = &dominio.Franja{Inicio: otra, Fin: otra.Add(2 * time.Hour)}
	cliente.Reservar(transferencia)

	primera, reintento, otraFranja := prueba.pedidos[0].Header.Get("Idempotency-Key"),
		prueba.pedidos[1].Header.Get("Idempotency-Key"), prueba.pedidos[2].Header.Get("Idempotency-Key")
	if primera == "" || primera != reintento || primera == otraFranja {
		t.Errorf("Se esperaba la misma clave en el reintento y otra con otra franja, pero se mandó %q, %q y %q", primera, reintento, otraFranja)
	}
}

func TestReservar_Errores(t *testing.T) {
	casos := []struct {
		nombre   string
		codigo   int
		rumbo    string
		esperado error
	}{
		{"franja sin lugar", http.StatusConflict, "SIN_DISPONIBILIDAD", puertos.ErrSinLugar},
		{"poca anticipación", http.StatusUnprocessableEntity, "ANTICIPACION_INSUFICIENTE", puertos.ErrSinLugar},
		{"franja ya empezada", http.StatusUnprocessableEntity, "HORARIO_EN_EL_PASADO", puertos.ErrSinLugar},
		{"excede la capacidad", http.StatusUnprocessableEntity, "EXCEDE_CAPACIDAD", puertos.ErrRechazoDelPedido},
		{"clave reutilizada", http.StatusConflict, "CLAVE_IDEMPOTENCIA_REUTILIZADA", puertos.ErrRechazoDelPedido},
		{"código que no conocemos", http.StatusBadRequest, "CODIGO_NUEVO", puertos.ErrRechazoDelPedido},
		{"tiempo agotado", http.StatusGatewayTimeout, "TIEMPO_AGOTADO", puertos.ErrNoDisponible},
		{"no disponible", http.StatusServiceUnavailable, "SERVICIO_NO_DISPONIBLE", puertos.ErrNoDisponible},
	}
	for _, caso := range casos {
		cliente, _ := nuevoCliente(t, problemaJSON(caso.codigo, caso.rumbo))

		n, err := cliente.Reservar(transferenciaConFranja())

		if !errors.Is(err, caso.esperado) {
			t.Errorf("%s: se esperaba %v, pero se obtuvo %v", caso.nombre, caso.esperado, err)
		}
		if caso.esperado == puertos.ErrRechazoDelPedido && n.Motivo == "" {
			t.Errorf("%s: se esperaba el motivo de Rumbo en la novedad", caso.nombre)
		}
	}
}

// Con el token vencido, Rumbo responde 401: pedimos uno nuevo y reintentamos una vez.
func TestPedir_TokenVencido(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer token-1" {
			problemaJSON(http.StatusUnauthorized, "NO_AUTENTICADO")(w, r)
			return
		}
		responderJSON(w, http.StatusCreated, trasladoProgramado)
	})

	_, err := cliente.Reservar(transferenciaConFranja())

	if err != nil || prueba.tokens != 2 {
		t.Errorf("Se esperaba reservar con un token nuevo, pero se obtuvo %v con %d tokens", err, prueba.tokens)
	}
}

// Con 429, respetamos el Retry-After: hasta que pase, no volvemos a llamar a Rumbo.
func TestPedir_DemasiadosPedidos(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		problemaJSON(http.StatusTooManyRequests, "DEMASIADOS_PEDIDOS")(w, r)
	})

	_, primero := cliente.Reservar(transferenciaConFranja())
	_, segundo := cliente.Reservar(transferenciaConFranja())

	if !errors.Is(primero, puertos.ErrNoDisponible) || !errors.Is(segundo, puertos.ErrNoDisponible) || len(prueba.pedidos) != 1 {
		t.Errorf("Se esperaba esperar sin volver a llamar, pero se obtuvo %v, %v y %d pedidos", primero, segundo, len(prueba.pedidos))
	}
}

// Después de 5 fallas seguidas el circuito se abre y no llamamos a Rumbo. Pasados los 30 segundos solo
// pasa la consulta de cambios, como prueba, y si responde se cierra.
func TestCircuitBreaker(t *testing.T) {
	anda := false
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		if !anda {
			problemaJSON(http.StatusServiceUnavailable, "SERVICIO_NO_DISPONIBLE")(w, r)
			return
		}
		responderJSON(w, http.StatusOK, `{"resultados":[],"pagina":1,"tamanio":100,"total":0}`)
	})

	for i := 0; i < fallasParaAbrir; i++ {
		cliente.Reservar(transferenciaConFranja())
	}
	_, err := cliente.Reservar(transferenciaConFranja())
	if !errors.Is(err, puertos.ErrNoDisponible) || len(prueba.pedidos) != fallasParaAbrir {
		t.Fatalf("Con el circuito abierto no tenía que llamar a Rumbo, pero hizo %d pedidos (%v)", len(prueba.pedidos), err)
	}

	// Pasan los 30 segundos y Rumbo vuelve: una reserva no pasa, la consulta de cambios sí.
	cliente.abiertoHasta = time.Now().Add(-time.Second)
	anda = true
	if _, err := cliente.Reservar(transferenciaConFranja()); !errors.Is(err, puertos.ErrNoDisponible) {
		t.Errorf("Con el circuito medio abierto una reserva no tenía que pasar, pero se obtuvo %v", err)
	}
	if _, _, err := cliente.Cambios(time.Time{}); err != nil {
		t.Fatalf("La consulta de cambios tenía que pasar como prueba, pero falló: %v", err)
	}
	if _, err := cliente.Franjas(time.Now()); err != nil {
		t.Errorf("Después de la prueba el circuito tenía que cerrarse, pero falló: %v", err)
	}
}

func TestCambios_TraeTodasLasPaginas(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pagina") == "1" {
			resultados := ""
			for i := 0; i < 100; i++ {
				if i > 0 {
					resultados += ","
				}
				resultados += fmt.Sprintf(`{"id":"sol-%d","referencia_externa":"trf-%d","estado":"PROGRAMADO","actualizado_en":"2026-11-19T08:30:00-03:00"}`, i, i)
			}
			responderJSON(w, http.StatusOK, `{"resultados":[`+resultados+`],"pagina":1,"tamanio":100,"total":101}`)
			return
		}
		responderJSON(w, http.StatusOK, `{"resultados":[{"id":"sol-100","referencia_externa":"trf-100","estado":"COMPLETADO","actualizado_en":"2026-11-20T11:40:00-03:00"}],"pagina":2,"tamanio":100,"total":101}`)
	})
	desde := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	novedades, hasta, err := cliente.Cambios(desde)

	if err != nil || len(novedades) != 101 || len(prueba.pedidos) != 2 {
		t.Fatalf("Se esperaban 101 novedades en 2 páginas, pero se obtuvo %d en %d (%v)", len(novedades), len(prueba.pedidos), err)
	}
	if !hasta.Equal(time.Date(2026, 11, 20, 14, 40, 0, 0, time.UTC)) {
		t.Errorf("Se esperaba seguir desde el último cambio, pero quedó %v", hasta)
	}
	if desdeEnviado := prueba.pedidos[0].URL.Query().Get("actualizado_desde"); desdeEnviado != "2026-11-19T08:59:59-03:00" {
		t.Errorf("Se esperaba pedir desde un segundo antes en hora de Argentina, pero se pidió %q", desdeEnviado)
	}
}

func TestCancelar(t *testing.T) {
	cliente, prueba := nuevoCliente(t, func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, `{"id":"sol-7f3a9c","referencia_externa":"trf-1","estado":"CANCELADO",
			"cancelacion":{"motivo":"Ya no hace falta","tardia":false,"cancelada_en":"2026-11-19T09:00:00-03:00"},
			"actualizado_en":"2026-11-19T09:00:00-03:00"}`)
	})
	transferencia := transferenciaConFranja()
	transferencia.IDRumbo = "sol-7f3a9c"

	n, err := cliente.Cancelar(transferencia, "Ya no hace falta")

	if err != nil || n.Estado != dominio.Cancelada {
		t.Fatalf("Se esperaba CANCELADA, pero se obtuvo %+v (%v)", n, err)
	}
	pedido := prueba.pedidos[0]
	if pedido.URL.Path != "/traslados/sol-7f3a9c/cancelacion" || pedido.Header.Get("Idempotency-Key") != "trf-1-cancelacion" ||
		prueba.cuerpos[0]["motivo"] != "Ya no hace falta" {
		t.Errorf("Se esperaba la cancelación con el motivo y su clave, pero se mandó %s %v", pedido.URL.Path, prueba.cuerpos[0])
	}
}

// Con el traslado en curso, Rumbo no deja cancelar.
func TestCancelar_EnCurso(t *testing.T) {
	cliente, _ := nuevoCliente(t, problemaJSON(http.StatusConflict, "ESTADO_NO_PERMITE_OPERACION"))
	transferencia := transferenciaConFranja()
	transferencia.IDRumbo = "sol-7f3a9c"

	_, err := cliente.Cancelar(transferencia, "Ya no hace falta")

	if !errors.Is(err, puertos.ErrEstadoNoPermite) {
		t.Errorf("Se esperaba ErrEstadoNoPermite, pero se obtuvo %v", err)
	}
}
