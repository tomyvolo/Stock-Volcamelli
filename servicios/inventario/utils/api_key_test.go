package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newRouter arma un router con el middleware y una ruta que responde 200.
func newRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(APIKeyRequired())
	router.GET("/api/products", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	return router
}

func request(router *gin.Engine, apiKey string) int {
	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

func TestAPIKey_NotConfiguredLetsEverythingPass(t *testing.T) {
	t.Setenv("INVENTARIO_API_KEY", "")
	router := newRouter()

	if code := request(router, ""); code != http.StatusOK {
		t.Errorf("Sin API key configurada se esperaba 200, pero se obtuvo: %d", code)
	}
}

func TestAPIKey_MissingOrWrong(t *testing.T) {
	t.Setenv("INVENTARIO_API_KEY", "clave-correcta")
	router := newRouter()

	if code := request(router, ""); code != http.StatusUnauthorized {
		t.Errorf("Sin header se esperaba 401, pero se obtuvo: %d", code)
	}
	if code := request(router, "otra-clave"); code != http.StatusUnauthorized {
		t.Errorf("Con una clave incorrecta se esperaba 401, pero se obtuvo: %d", code)
	}
}

func TestAPIKey_Correct(t *testing.T) {
	t.Setenv("INVENTARIO_API_KEY", "clave-correcta")
	router := newRouter()

	if code := request(router, "clave-correcta"); code != http.StatusOK {
		t.Errorf("Con la clave correcta se esperaba 200, pero se obtuvo: %d", code)
	}
}
