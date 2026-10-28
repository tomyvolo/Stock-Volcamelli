package utils

import "github.com/gin-gonic/gin"

// Códigos de error del contrato (docs/contracts/openapi.yaml).
const (
	CodeInvalidRequest  = "INVALID_REQUEST"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeProductNotFound = "PRODUCT_NOT_FOUND"
	CodeInternal        = "INTERNAL_ERROR"
)

// ErrorResponse arma el cuerpo de error del contrato: {code, message}.
func ErrorResponse(code, message string) gin.H {
	return gin.H{
		"code":    code,
		"message": message,
	}
}
