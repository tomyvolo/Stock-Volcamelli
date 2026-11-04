package reservations

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"inventario/logs"
	"inventario/models/reservations"
	services "inventario/services/reservations"
	"inventario/utils"
)

const stage = "inventario"

type reserveRequest struct {
	TransferID          string `json:"transferId" binding:"required"`
	ProductID           uint   `json:"productId" binding:"required"`
	OriginBranchID      uint   `json:"originBranchId" binding:"required"`
	DestinationBranchID uint   `json:"destinationBranchId" binding:"required"`
	Quantity            int    `json:"quantity" binding:"required"`
}

// ReservationsController atiende las rutas /internal, que usa solo Transferencias.
// No pasan por el gateway: Transferencias llama directo, por el NGINX de Inventario.
type ReservationsController struct {
	Service services.ReservationsService
	Logs    *logs.Store
}

// Reserve responde POST /internal/reservations.
func (c *ReservationsController) Reserve(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	var req reserveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Son obligatorios transferId, productId, originBranchId, destinationBranchId y quantity."))
		return
	}

	reservation, err := c.Service.Reserve(reservations.ReservationModel{
		TransferID: req.TransferID, ProductID: req.ProductID, OriginBranchID: req.OriginBranchID,
		DestinationBranchID: req.DestinationBranchID, Quantity: req.Quantity,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidReservation):
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, err.Error()+"."))
		case errors.Is(err, services.ErrProductNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeProductNotFound, fmt.Sprintf("No existe el producto %d.", req.ProductID)))
		case errors.Is(err, services.ErrInsufficientStock):
			c.Logs.Write(trackID, stage, "warn", fmt.Sprintf("reserva de la transferencia %s rechazada: no hay %d disponibles del producto %d en la sucursal %d",
				req.TransferID, req.Quantity, req.ProductID, req.OriginBranchID))
			ctx.JSON(http.StatusConflict, utils.ErrorResponse(utils.CodeInsufficientStock, "No hay stock disponible suficiente en el origen."))
		case errors.Is(err, services.ErrReservationReleased):
			c.Logs.Write(trackID, stage, "warn", "reserva rechazada: la transferencia "+req.TransferID+" ya se había liberado")
			ctx.JSON(http.StatusConflict, utils.ErrorResponse(utils.CodeReservationReleased, "La reserva de esa transferencia ya fue liberada."))
		default:
			c.Logs.Write(trackID, stage, "error", "reservar: "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo reservar"))
		}
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("reserva de la transferencia %s: %d del producto %d en la sucursal %d (%d ms)",
		reservation.TransferID, reservation.Quantity, reservation.ProductID, reservation.OriginBranchID, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, reservation)
}

// Release responde POST /internal/reservations/:transferId/release.
func (c *ReservationsController) Release(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")
	transferID := ctx.Param("transferId")

	reservation, err := c.Service.Release(transferID)
	if err != nil {
		if errors.Is(err, services.ErrReservationConfirmed) {
			c.Logs.Write(trackID, stage, "warn", "no se libera la transferencia "+transferID+": ya estaba confirmada")
			ctx.JSON(http.StatusConflict, utils.ErrorResponse(utils.CodeReservationConfirmed, "La reserva de esa transferencia ya fue confirmada."))
			return
		}
		c.Logs.Write(trackID, stage, "error", "liberar: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo liberar"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("reserva de la transferencia %s liberada (%d ms)", transferID, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, reservation)
}
