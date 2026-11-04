package stock

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"inventario/logs"
	"inventario/models/stock"
	services "inventario/services/stock"
	"inventario/utils"
)

const stage = "inventario"

type adjustmentRequest struct {
	Type     string `json:"type" binding:"required"`
	Quantity int    `json:"quantity" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
}

type stockChangeRequest struct {
	Quantity int    `json:"quantity" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
}

type StockController struct {
	Service services.StockService
	Logs    *logs.Store
}

// GetByBranch responde GET /api/branches/:branchId/stock y, con ?lowStock=true, solo el stock bajo.
func (c *StockController) GetByBranch(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	branchID, err := utils.ParseIDParam(ctx, "branchId")
	if err != nil {
		return
	}
	lowStock, err := strconv.ParseBool(ctx.DefaultQuery("lowStock", "false"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El parámetro lowStock tiene que ser true o false."))
		return
	}

	list, err := c.Service.GetBranchStock(branchID, lowStock)
	if err != nil {
		c.Logs.Write(trackID, stage, "error", "stock de sucursal: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo obtener el stock"))
		return
	}

	filter := ""
	if lowStock {
		filter = " con stock bajo"
	}
	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("stock de la sucursal %d: %d productos%s (%d ms)",
		branchID, len(list), filter, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{
		"branchId": branchID,
		"items":    list,
	})
}

// GetByProduct responde GET /api/branches/:branchId/products/:productId/stock.
func (c *StockController) GetByProduct(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	branchID, err := utils.ParseIDParam(ctx, "branchId")
	if err != nil {
		return
	}
	productID, err := utils.ParseIDParam(ctx, "productId")
	if err != nil {
		return
	}

	row, err := c.Service.GetProductStock(branchID, productID)
	if err != nil {
		if errors.Is(err, services.ErrProductNotFound) {
			c.Logs.Write(trackID, stage, "warn", fmt.Sprintf("stock de producto: no existe el producto %d", productID))
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeProductNotFound, fmt.Sprintf("No existe el producto %d.", productID)))
			return
		}
		c.Logs.Write(trackID, stage, "error", "stock de producto: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo obtener el stock"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("stock del producto %d en la sucursal %d: %d disponibles (%d ms)",
		productID, branchID, row.Quantity, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, row)
}

// Adjust responde POST /api/branches/:branchId/products/:productId/adjustments.
// El gateway deja pasar al director y al admin de sucursal; que el admin sea de esa sucursal lo controla el service.
func (c *StockController) Adjust(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	branchID, err := utils.ParseIDParam(ctx, "branchId")
	if err != nil {
		return
	}
	productID, err := utils.ParseIDParam(ctx, "productId")
	if err != nil {
		return
	}
	var req adjustmentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Son obligatorios type (ENTRADA o SALIDA), quantity y reason."))
		return
	}

	userRole := ctx.GetHeader("X-User-Role")
	var userBranch *uint
	if header := ctx.GetHeader("X-User-Branch"); header != "" {
		if branch, err := strconv.ParseUint(header, 10, 32); err == nil {
			value := uint(branch)
			userBranch = &value
		}
	}

	row, err := c.Service.Adjust(userRole, userBranch, branchID, productID, req.Type, req.Quantity, req.Reason, trackID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrNotAllowed):
			c.Logs.Write(trackID, stage, "warn", fmt.Sprintf("ajuste rechazado: un %s no puede ajustar la sucursal %d", userRole, branchID))
			ctx.JSON(http.StatusForbidden, utils.ErrorResponse(utils.CodeForbidden, "No tenés permiso para ajustar el stock de esta sucursal."))
		case errors.Is(err, services.ErrInvalidAdjustment):
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El tipo tiene que ser ENTRADA o SALIDA, la cantidad mayor a 0 y el motivo no puede estar vacío ni pasar los 200 caracteres."))
		case errors.Is(err, services.ErrBranchNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeBranchNotFound, fmt.Sprintf("No existe la sucursal %d.", branchID)))
		case errors.Is(err, services.ErrProductNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeProductNotFound, fmt.Sprintf("No existe el producto %d.", productID)))
		case errors.Is(err, services.ErrBelowReserved):
			ctx.JSON(http.StatusConflict, utils.ErrorResponse(utils.CodeInsufficientStock, "La salida dejaría el total por debajo de lo reservado o en negativo."))
		case errors.Is(err, services.ErrBranchesUnavailable):
			c.Logs.Write(trackID, stage, "error", "ajuste: "+err.Error())
			ctx.JSON(http.StatusServiceUnavailable, utils.ErrorResponse(utils.CodeServiceUnavailable, "No se pudo validar la sucursal. Probá de nuevo en unos minutos."))
		default:
			c.Logs.Write(trackID, stage, "error", "ajuste: "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo hacer el ajuste"))
		}
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("ajuste de %s de %d del producto %d en la sucursal %d: total %d (%d ms)",
		req.Type, req.Quantity, productID, branchID, row.Total, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{
		"branchId":  row.BranchID,
		"productId": row.ProductID,
		"total":     row.Total,
		"reserved":  row.Reserved,
		"available": row.Total - row.Reserved,
	})
}

// Increase responde POST /api/branches/:branchId/products/:productId/stock/increase.
// Increase y Decrease existen solo en Inventario publicado, para el grupo consumidor (docs/contracts/openapi.yaml).
func (c *StockController) Increase(ctx *gin.Context) {
	c.changeStock(ctx, "aumento", c.Service.Increase)
}

// Decrease responde POST /api/branches/:branchId/products/:productId/stock/decrease.
func (c *StockController) Decrease(ctx *gin.Context) {
	c.changeStock(ctx, "reducción", c.Service.Decrease)
}

type stockChange func(branchID, productID uint, quantity int, reason, trackID string) (stock.StockModel, error)

func (c *StockController) changeStock(ctx *gin.Context, name string, change stockChange) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	branchID, err := utils.ParseIDParam(ctx, "branchId")
	if err != nil {
		return
	}
	productID, err := utils.ParseIDParam(ctx, "productId")
	if err != nil {
		return
	}
	var req stockChangeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Son obligatorios quantity y reason."))
		return
	}

	result, err := change(branchID, productID, req.Quantity, req.Reason, trackID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAdjustment):
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "La cantidad tiene que ser mayor a 0 y el motivo no puede estar vacío ni pasar los 200 caracteres."))
		case errors.Is(err, services.ErrBranchNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeBranchNotFound, fmt.Sprintf("No existe la sucursal %d.", branchID)))
		case errors.Is(err, services.ErrProductNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeProductNotFound, fmt.Sprintf("No existe el producto %d.", productID)))
		case errors.Is(err, services.ErrBelowReserved):
			ctx.JSON(http.StatusConflict, utils.ErrorResponse(utils.CodeInsufficientStock,
				fmt.Sprintf("El stock disponible del producto %d en la sucursal %d no alcanza para restar %d.", productID, branchID, req.Quantity)))
		default:
			c.Logs.Write(trackID, stage, "error", name+" de stock: "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "Ocurrió un error inesperado. Probá de nuevo en unos minutos."))
		}
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("%s de %d del producto %d en la sucursal %d: disponible %d (%d ms)",
		name, req.Quantity, productID, branchID, result.Quantity, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, result)
}

// Movements responde GET /api/branches/:branchId/products/:productId/movements?page=1&size=20.
func (c *StockController) Movements(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	branchID, err := utils.ParseIDParam(ctx, "branchId")
	if err != nil {
		return
	}
	productID, err := utils.ParseIDParam(ctx, "productId")
	if err != nil {
		return
	}
	page, size, err := utils.ParsePagination(ctx)
	if err != nil {
		return
	}

	result, err := c.Service.GetMovements(branchID, productID, page, size)
	if err != nil {
		if errors.Is(err, services.ErrProductNotFound) {
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeProductNotFound, fmt.Sprintf("No existe el producto %d.", productID)))
			return
		}
		c.Logs.Write(trackID, stage, "error", "movimientos: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudieron obtener los movimientos"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("movimientos del producto %d en la sucursal %d: %d de %d (%d ms)",
		productID, branchID, len(result.Movements), result.Total, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{
		"items": result.Movements,
		"page":  page,
		"size":  size,
		"total": result.Total,
	})
}
