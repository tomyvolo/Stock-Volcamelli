package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func routerWithLimit(limit int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(limit))
	router.GET("/api/branches", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	return router
}

func get(router *gin.Engine, ip string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/branches", nil)
	request.RemoteAddr = ip + ":40000"
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
