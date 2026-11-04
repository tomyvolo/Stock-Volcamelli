package utils

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// APIKeyRequired exige el header X-API-Key cuando INVENTARIO_API_KEY está definida, que es el caso de
// Inventario publicado solo. En Docker Compose no se define: Inventario no tiene puerto abierto y solo
// le llegan pedidos del gateway, que ya validó el token.
func APIKeyRequired() gin.HandlerFunc {
	apiKey := os.Getenv("INVENTARIO_API_KEY")
	return func(ctx *gin.Context) {
		if apiKey == "" {
			ctx.Next()
			return
		}
		// La comparación tarda lo mismo acierte o no, para no dar pistas sobre la clave.
		if subtle.ConstantTimeCompare([]byte(ctx.GetHeader("X-API-Key")), []byte(apiKey)) != 1 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse(CodeUnauthorized, "Falta la API key o no es válida."))
			return
		}
		ctx.Next()
	}
}
