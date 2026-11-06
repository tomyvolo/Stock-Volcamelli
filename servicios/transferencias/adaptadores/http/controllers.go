// Package http es el adaptador de entrada: recibe los pedidos del gateway con Gin y llama a los casos de uso.
package http

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"

	"transferencias/aplicacion"
	"transferencias/dominio"
	"transferencias/puertos"
)

type crearRequest struct {
	ProductID           uint       `json:"productId" binding:"required"`
	Quantity            int        `json:"quantity" binding:"required"`
	OriginBranchID      uint       `json:"originBranchId" binding:"required"`
	DestinationBranchID uint       `json:"destinationBranchId" binding:"required"`
	DesiredArrival      *time.Time `json:"desiredArrival"`
	ToleranceMinutes    int        `json:"toleranceMinutes"`
	Notes               string     `json:"notes"`
}

type cancelarRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type responderRequest struct {
	Accept *bool `json:"accept" binding:"required"`
}

type Controllers struct {
	Servicio *aplicacion.Servicio
	Registro puertos.Registro
}

func errorJSON(code, message string) gin.H {
	return gin.H{"code": code, "message": message}
}

// usuario arma quién hace el pedido con los headers que agrega el gateway, sacados del token.
func usuario(ctx *gin.Context) aplicacion.Usuario {
	u := aplicacion.Usuario{Rol: ctx.GetHeader("X-User-Role")}
	if id, err := strconv.ParseUint(ctx.GetHeader("X-User-ID"), 10, 32); err == nil {
		u.ID = uint(id)
	}
	if sucursal, err := strconv.ParseUint(ctx.GetHeader("X-User-Branch"), 10, 32); err == nil {
		value := uint(sucursal)
		u.SucursalID = &value
	}
	return u
}

// Crear responde POST /api/transfers. Responde 201 también con SIN_STOCK o FALLIDA: la transferencia
// se creó y el estado dice qué pasó.
func (c *Controllers) Crear(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	var req crearRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST",
			"Son obligatorios productId, quantity, originBranchId y destinationBranchId. desiredArrival va en formato ISO 8601."))
		return
	}

	t, err := c.Servicio.Crear(aplicacion.Pedido{
		ProductoID: req.ProductID, Cantidad: req.Quantity, OrigenID: req.OriginBranchID, DestinoID: req.DestinationBranchID,
		HoraLlegada: req.DesiredArrival, ToleranciaMinutos: req.ToleranceMinutes, Observaciones: req.Notes,
	}, usuario(ctx), trackID)
	if err != nil {
		c.responderError(ctx, trackID, "crear transferencia", err)
		return
	}

	c.Registro.Escribir(trackID, "info", fmt.Sprintf("transferencia %s creada: %d del producto %d de la sucursal %d a la %d, %s (%d ms)",
		t.ID, t.Cantidad, t.ProductoID, t.OrigenID, t.DestinoID, t.Estado, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusCreated, aJSON(t))
}

// Listar responde GET /api/transfers?branchId=1&status=RESERVADA&page=1&size=20.
func (c *Controllers) Listar(ctx *gin.Context) {
	start := time.Now()
	filtro := puertos.Filtro{Estado: dominio.Estado(ctx.Query("status")), Pagina: 1, Tamanio: 20}
	if valor := ctx.Query("branchId"); valor != "" {
		sucursal, err := strconv.ParseUint(valor, 10, 32)
		if err != nil || sucursal == 0 {
			ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", "El parámetro branchId tiene que ser un número mayor a 0."))
			return
		}
		filtro.SucursalID = uint(sucursal)
	}
	if pagina, err := strconv.Atoi(ctx.DefaultQuery("page", "1")); err != nil || pagina < 1 {
		ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", "El parámetro page tiene que ser un número mayor a 0."))
		return
	} else {
		filtro.Pagina = pagina
	}
	if tamanio, err := strconv.Atoi(ctx.DefaultQuery("size", "20")); err != nil || tamanio < 1 || tamanio > 100 {
		ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", "El parámetro size tiene que ser un número entre 1 y 100."))
		return
	} else {
		filtro.Tamanio = tamanio
	}

	lista, total, err := c.Servicio.Listar(filtro)
	if err != nil {
		c.responderError(ctx, ctx.GetString("track_id"), "listar transferencias", err)
		return
	}
	items := make([]transferenciaJSON, 0, len(lista))
	for _, t := range lista {
		items = append(items, aJSON(t))
	}
	c.Registro.Escribir(ctx.GetString("track_id"), "info", fmt.Sprintf("listar transferencias: página %d con %d de %d (%d ms)",
		filtro.Pagina, len(items), total, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{"items": items, "page": filtro.Pagina, "size": filtro.Tamanio, "total": total})
}

// Obtener responde GET /api/transfers/:transferId.
func (c *Controllers) Obtener(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")
	t, err := c.Servicio.Obtener(ctx.Param("transferId"))
	if err != nil {
		c.responderError(ctx, trackID, "ver transferencia", err)
		return
	}
	c.Registro.Escribir(trackID, "info", fmt.Sprintf("ver transferencia %s: %s (%d ms)", t.ID, t.Estado, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, aJSON(t))
}

// Cancelar responde POST /api/transfers/:transferId/cancel. Si Rumbo ya tenía el pedido, la cancelación
// se le manda en segundo plano y el estado cambia cuando Rumbo responde.
func (c *Controllers) Cancelar(ctx *gin.Context) {
	trackID := ctx.GetString("track_id")
	var req cancelarRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", "Es obligatorio reason, el motivo de la cancelación."))
		return
	}
	t, err := c.Servicio.Cancelar(ctx.Param("transferId"), usuario(ctx), req.Reason)
	if err != nil {
		c.responderError(ctx, trackID, "cancelar transferencia", err)
		return
	}
	c.Registro.Escribir(trackID, "info", fmt.Sprintf("cancelación pedida para la transferencia %s: %s", t.ID, req.Reason))
	ctx.JSON(http.StatusOK, aJSON(t))
}

// Responder responde POST /api/transfers/:transferId/schedule-response con {"accept": true|false}.
func (c *Controllers) Responder(ctx *gin.Context) {
	trackID := ctx.GetString("track_id")
	var req responderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", "Es obligatorio accept, true o false."))
		return
	}
	t, err := c.Servicio.Responder(ctx.Param("transferId"), usuario(ctx), *req.Accept)
	if err != nil {
		c.responderError(ctx, trackID, "responder horario", err)
		return
	}
	c.Registro.Escribir(trackID, "info", fmt.Sprintf("respuesta al horario propuesto de la transferencia %s: acepta=%t", t.ID, *req.Accept))
	ctx.JSON(http.StatusOK, aJSON(t))
}

// responderError traduce los errores del centro a la respuesta HTTP.
func (c *Controllers) responderError(ctx *gin.Context, trackID, accion string, err error) {
	switch {
	case errors.Is(err, dominio.ErrCantidadInvalida), errors.Is(err, dominio.ErrMismaSucursal),
		errors.Is(err, dominio.ErrHoraPasada), errors.Is(err, dominio.ErrToleranciaMala):
		ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", capitalizar(err.Error())))
	case errors.Is(err, aplicacion.ErrSinPermiso):
		c.Registro.Escribir(trackID, "warn", accion+" rechazado: "+err.Error())
		ctx.JSON(http.StatusForbidden, errorJSON("FORBIDDEN", "Solo podés operar las transferencias que salen de tu sucursal o llegan a ella."))
	case errors.Is(err, aplicacion.ErrNoExiste):
		ctx.JSON(http.StatusNotFound, errorJSON("TRANSFER_NOT_FOUND", "No existe esa transferencia."))
	case errors.Is(err, aplicacion.ErrSucursalInexistente):
		ctx.JSON(http.StatusNotFound, errorJSON("BRANCH_NOT_FOUND", "El origen o el destino no es una sucursal que exista."))
	case errors.Is(err, aplicacion.ErrProductoInexistente):
		ctx.JSON(http.StatusNotFound, errorJSON("PRODUCT_NOT_FOUND", "No existe ese producto."))
	case errors.Is(err, dominio.ErrNoCancelable):
		ctx.JSON(http.StatusConflict, errorJSON("NOT_CANCELLABLE", "La transferencia ya no se puede cancelar: está en curso, terminó o ya se pidió la cancelación."))
	case errors.Is(err, dominio.ErrNoHayPropuesta):
		ctx.JSON(http.StatusConflict, errorJSON("NO_PROPOSAL", "La transferencia no tiene un horario propuesto para responder."))
	case errors.Is(err, dominio.ErrPropuestaVencida):
		ctx.JSON(http.StatusConflict, errorJSON("PROPOSAL_EXPIRED", "El plazo para responder el horario propuesto ya venció."))
	case errors.Is(err, aplicacion.ErrNoDisponible):
		c.Registro.Escribir(trackID, "error", accion+": "+err.Error())
		ctx.JSON(http.StatusServiceUnavailable, errorJSON("SERVICE_UNAVAILABLE", "No se pudo crear la transferencia porque Sucursales o Inventario no respondió. Probá de nuevo en unos minutos."))
	default:
		c.Registro.Escribir(trackID, "error", accion+": "+err.Error())
		ctx.JSON(http.StatusInternalServerError, errorJSON("INTERNAL_ERROR", "Ocurrió un error inesperado."))
	}
}

// capitalizar convierte un error del dominio ("la cantidad tiene que...") en un mensaje para el usuario.
func capitalizar(texto string) string {
	letras := []rune(texto)
	if len(letras) == 0 {
		return texto
	}
	letras[0] = unicode.ToUpper(letras[0])
	return string(letras) + "."
}
