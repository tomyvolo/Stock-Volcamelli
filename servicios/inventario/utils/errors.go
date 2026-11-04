package utils

import "github.com/gin-gonic/gin"

// Códigos de error del contrato (docs/contracts/openapi.yaml).
const (
	CodeInvalidRequest    = "INVALID_REQUEST"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeProductNotFound   = "PRODUCT_NOT_FOUND"
	CodeBranchNotFound    = "BRANCH_NOT_FOUND"
	CodeInsufficientStock = "INSUFFICIENT_STOCK"
	CodeInternal          = "INTERNAL_ERROR"
)

// Códigos de las operaciones que no están en el contrato: ajustes, alta de productos y reservas.
const (
	CodeForbidden            = "FORBIDDEN"
	CodeReservationReleased  = "RESERVATION_RELEASED"
	CodeReservationConfirmed = "RESERVATION_CONFIRMED"
	CodeServiceUnavailable   = "SERVICE_UNAVAILABLE"
)

// ErrorResponse arma el cuerpo de error del contrato: {code, message}.
func ErrorResponse(code, message string) gin.H {
	return gin.H{
		"code":    code,
		"message": message,
	}
}
