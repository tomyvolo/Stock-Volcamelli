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

// ParsePagination lee page (desde 1) y size (de 1 a 100). Si alguno es inválido, responde 400 y devuelve error.
// page se lee de 32 bits para que (page-1)*size nunca desborde.
func ParsePagination(ctx *gin.Context) (int, int, error) {
	page64, err := strconv.ParseInt(ctx.DefaultQuery("page", "1"), 10, 32)
	page := int(page64)
	if err != nil || page < 1 {
		ctx.JSON(http.StatusBadRequest, ErrorResponse(CodeInvalidRequest, "El parámetro page tiene que ser un número mayor a 0."))
		return 0, 0, errors.New("page inválido")
	}
	size, err := strconv.Atoi(ctx.DefaultQuery("size", "20"))
	if err != nil || size < 1 || size > 100 {
		ctx.JSON(http.StatusBadRequest, ErrorResponse(CodeInvalidRequest, "El parámetro size tiene que ser un número entre 1 y 100."))
		return 0, 0, errors.New("size inválido")
	}
	return page, size, nil
}
