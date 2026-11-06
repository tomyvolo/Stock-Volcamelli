package utils

import "github.com/gin-gonic/gin"

// Códigos de error, con el mismo formato que el resto de los servicios.
const (
	CodeInvalidRequest = "INVALID_REQUEST"
	CodeBranchNotFound = "BRANCH_NOT_FOUND"
	CodeInternal       = "INTERNAL_ERROR"
)

// ErrorResponse arma el cuerpo de error: {code, message}.
func ErrorResponse(code, message string) gin.H {
	return gin.H{
		"code":    code,
		"message": message,
	}
}
