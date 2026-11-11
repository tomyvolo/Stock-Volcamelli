package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// proxyIP es la IP del NGINX del frontend en los tests.
const proxyIP = "10.0.0.100"

func routerWithLimit(limit int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.SetTrustedProxies(nil)
	router.Use(RateLimit(limit, func(ip string) bool { return ip == proxyIP }))
	router.GET("/api/branches", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	return router
}

func get(router *gin.Engine, ip string, headers ...string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/branches", nil)
	request.RemoteAddr = ip + ":40000"
	for i := 0; i+1 < len(headers); i += 2 {
		request.Header.Set(headers[i], headers[i+1])
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// Pasado el límite, la misma IP recibe 429 con Retry-After; otra IP sigue pasando.
func TestRateLimit_CortaPorIP(t *testing.T) {
	router := routerWithLimit(3)

	for i := 1; i <= 3; i++ {
		if response := get(router, "10.0.0.1"); response.Code != http.StatusOK {
			t.Fatalf("El pedido %d tenía que pasar, pero respondió %d", i, response.Code)
		}
	}
	response := get(router, "10.0.0.1")
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
		t.Errorf("El cuarto pedido tenía que recibir 429 con Retry-After, pero respondió %d (%q)", response.Code, response.Header().Get("Retry-After"))
	}
	if response := get(router, "10.0.0.2"); response.Code != http.StatusOK {
		t.Errorf("Otra IP tenía que pasar, pero respondió %d", response.Code)
	}
}

// Con límite 0 no se corta nunca (pruebas de carga).
func TestRateLimit_CeroNoLimita(t *testing.T) {
	router := routerWithLimit(0)

	for i := 1; i <= 100; i++ {
		if response := get(router, "10.0.0.1"); response.Code != http.StatusOK {
			t.Fatalf("Sin límite, el pedido %d tenía que pasar, pero respondió %d", i, response.Code)
		}
	}
}

// Un cliente que llama directo al gateway no se saltea el límite cambiando X-Forwarded-For o X-Real-IP.
func TestRateLimit_IgnoraLosEncabezadosDelCliente(t *testing.T) {
	router := routerWithLimit(1)

	codes := []int{
		get(router, "10.0.0.1", "X-Forwarded-For", "1.1.1.1").Code,
		get(router, "10.0.0.1", "X-Forwarded-For", "1.1.1.1").Code,
		get(router, "10.0.0.1", "X-Forwarded-For", "2.2.2.2").Code,
		get(router, "10.0.0.1", "X-Real-IP", "3.3.3.3").Code,
	}

	if codes[0] != http.StatusOK || codes[1] != http.StatusTooManyRequests || codes[2] != http.StatusTooManyRequests || codes[3] != http.StatusTooManyRequests {
		t.Errorf("Se esperaba 200 y después 429 aunque cambie el encabezado, pero respondió %v", codes)
	}
}

// Por el NGINX del frontend, cada navegador tiene su propio cupo, según la IP que pone NGINX en X-Real-IP.
func TestRateLimit_ProxyDeConfianza(t *testing.T) {
	router := routerWithLimit(1)

	primero := get(router, proxyIP, "X-Real-IP", "192.168.1.10").Code
	repetido := get(router, proxyIP, "X-Real-IP", "192.168.1.10").Code
	otro := get(router, proxyIP, "X-Real-IP", "192.168.1.20").Code

	if primero != http.StatusOK || repetido != http.StatusTooManyRequests || otro != http.StatusOK {
		t.Errorf("Se esperaba 200, 429 y 200 para otro navegador, pero respondió %d, %d y %d", primero, repetido, otro)
	}
}
