package utils

import "github.com/gin-gonic/gin"

// Códigos de error del contrato (docs/contracts/openapi.yaml).
const (
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeInternal           = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// ErrorResponse arma el cuerpo de error del contrato: {code, message}.
func ErrorResponse(code, message string) gin.H {
	return gin.H{
		"code":    code,
		"message": message,
	}
}
