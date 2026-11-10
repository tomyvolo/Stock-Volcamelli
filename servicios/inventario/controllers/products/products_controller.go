package products

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"inventario/logs"
	"inventario/models/products"
	services "inventario/services/products"
	"inventario/utils"
)

const stage = "inventario"

type productRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Supplier    *string `json:"supplier"`
}

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

// Search responde GET /api/products/search?q=harina&supplier=Molinos del Sur&sort=nombre&page=1&size=20.
// El texto es obligatorio. Si Solr no responde, devuelve 502 y no una lista vacía, que parecería que no hay productos.
// sort es relevancia (si no viene) o nombre, de la A a la Z.
func (c *ProductsController) Search(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	text := strings.TrimSpace(ctx.Query("q"))
	if text == "" {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El parámetro q es obligatorio."))
		return
	}
	sort := ctx.DefaultQuery("sort", products.SortRelevance)
	if sort != products.SortRelevance && sort != products.SortName {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El parámetro sort tiene que ser relevancia o nombre."))
		return
	}
	page, size, err := utils.ParsePagination(ctx)
	if err != nil {
		return
	}

	result, err := c.Service.SearchProducts(products.SearchQuery{
		Text: text, Supplier: strings.TrimSpace(ctx.Query("supplier")), Page: page, Size: size, Sort: sort,
	})
	if err != nil {
		c.Logs.Write(trackID, stage, "error", "buscar productos: "+err.Error())
		ctx.JSON(http.StatusBadGateway, utils.ErrorResponse(utils.CodeServiceUnavailable, "La búsqueda no está disponible."))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("buscar %q: página %d con %d de %d resultados (%d ms)",
		text, page, len(result.Results), result.Total, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{
		"results": result.Results,
		"page":    page,
		"size":    size,
		"total":   result.Total,
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

// Create responde POST /api/products. El gateway solo la deja pasar con el token de un director.
func (c *ProductsController) Create(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	var req productRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Es obligatorio name. description y supplier son opcionales."))
		return
	}

	product, err := c.Service.CreateProduct(req.Name, req.Description, req.Supplier, trackID)
	if err != nil {
		if errors.Is(err, services.ErrNameRequired) {
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El nombre del producto no puede estar vacío."))
			return
		}
		c.Logs.Write(trackID, stage, "error", "alta de producto: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo crear el producto"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("producto %d creado (%d ms)", product.ID, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusCreated, product)
}

// Update responde PUT /api/products/:productId. El gateway solo la deja pasar con el token de un director.
func (c *ProductsController) Update(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	id, err := utils.ParseIDParam(ctx, "productId")
	if err != nil {
		return
	}
	var req productRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Es obligatorio name. description y supplier son opcionales."))
		return
	}

	product, err := c.Service.UpdateProduct(id, req.Name, req.Description, req.Supplier, trackID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrProductNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeProductNotFound, fmt.Sprintf("No existe el producto %d.", id)))
		case errors.Is(err, services.ErrNameRequired):
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El nombre del producto no puede estar vacío."))
		default:
			c.Logs.Write(trackID, stage, "error", "modificar producto: "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo modificar el producto"))
		}
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("producto %d modificado (%d ms)", id, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, product)
}
