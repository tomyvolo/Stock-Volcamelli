package utils

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParseIDParam lee un id de la URL. Si no es un número mayor a 0, responde 400 y devuelve error.
func ParseIDParam(ctx *gin.Context, name string) (uint, error) {
	id, err := strconv.ParseUint(ctx.Param(name), 10, 32)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, ErrorResponse(CodeInvalidRequest, "El "+name+" tiene que ser un número mayor a 0."))
		return 0, errors.New(name + " inválido")
	}
	return uint(id), nil
}
