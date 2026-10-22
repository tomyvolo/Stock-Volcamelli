package products

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"inventario/logs"
	services "inventario/services/products"
	"inventario/utils"
)

const stage = "inventario"

type ProductsController struct {
	Service services.ProductsService
	Logs    *logs.Store
}

// List responde GET /api/products?page=1&size=20.
func (c *ProductsController) List(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	page, size, err := utils.ParsePagination(ctx)
	if err != nil {
		return
	}

	result, err := c.Service.GetProducts(page, size)
	if err != nil {
		c.Logs.Write(trackID, stage, "error", "listar productos: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudieron obtener los productos"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("listar productos: página %d con %d de %d productos (%d ms)",
		page, len(result.Products), result.Total, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{
		"items": result.Products,
		"page":  page,
		"size":  size,
		"total": result.Total,
	})
}

// GetByID responde GET /api/products/:productId.
func (c *ProductsController) GetByID(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	id, err := utils.ParseIDParam(ctx, "productId")
	if err != nil {
		return
	}

	product, err := c.Service.GetProduct(id)
	if err != nil {
		if errors.Is(err, services.ErrProductNotFound) {
			c.Logs.Write(trackID, stage, "warn", fmt.Sprintf("ver producto: no existe el producto %d", id))
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeProductNotFound, fmt.Sprintf("No existe el producto %d.", id)))
			return
		}
		c.Logs.Write(trackID, stage, "error", "ver producto: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo obtener el producto"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("ver producto %d (%d ms)", id, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, product)
}
