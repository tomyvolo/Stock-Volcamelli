package stock

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"inventario/logs"
	services "inventario/services/stock"
	"inventario/utils"
)

const stage = "inventario"

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
