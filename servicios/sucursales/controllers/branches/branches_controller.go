package branches

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"sucursales/logs"
	services "sucursales/services/branches"
	"sucursales/utils"
)

const stage = "sucursales"

type branchRequest struct {
	Name    string `json:"name" binding:"required"`
	Address string `json:"address" binding:"required"`
}

type BranchesController struct {
	Service services.BranchesService
	Logs    *logs.Store
}

// List responde GET /api/branches.
func (c *BranchesController) List(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	list, err := c.Service.GetBranches()
	if err != nil {
		c.Logs.Write(trackID, stage, "error", "listar sucursales: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudieron obtener las sucursales"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("listar sucursales: %d sucursales (%d ms)", len(list), time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{"items": list})
}

// GetByID responde GET /api/branches/:branchId. Lo usan el frontend y Transferencias.
func (c *BranchesController) GetByID(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	id, err := utils.ParseIDParam(ctx, "branchId")
	if err != nil {
		return
	}

	branch, err := c.Service.GetBranch(id)
	if err != nil {
		c.respondError(ctx, trackID, "ver sucursal", id, err)
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("ver sucursal %d (%d ms)", id, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, branch)
}

// Create responde POST /api/branches. El gateway ya controló que lo pida un director.
func (c *BranchesController) Create(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	var req branchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Son obligatorios name y address."))
		return
	}

	branch, err := c.Service.CreateBranch(req.Name, req.Address)
	if err != nil {
		c.respondError(ctx, trackID, "alta de sucursal", 0, err)
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("sucursal %d creada (%d ms)", branch.ID, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusCreated, branch)
}

// Update responde PUT /api/branches/:branchId. El gateway ya controló que lo pida un director.
func (c *BranchesController) Update(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	id, err := utils.ParseIDParam(ctx, "branchId")
	if err != nil {
		return
	}
	var req branchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Son obligatorios name y address."))
		return
	}

	branch, err := c.Service.UpdateBranch(id, req.Name, req.Address)
	if err != nil {
		c.respondError(ctx, trackID, "modificar sucursal", id, err)
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("sucursal %d modificada (%d ms)", id, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, branch)
}

// respondError traduce los errores del service a la respuesta HTTP.
func (c *BranchesController) respondError(ctx *gin.Context, trackID, action string, id uint, err error) {
	switch {
	case errors.Is(err, services.ErrBranchNotFound):
		c.Logs.Write(trackID, stage, "warn", fmt.Sprintf("%s: no existe la sucursal %d", action, id))
		ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeBranchNotFound, fmt.Sprintf("No existe la sucursal %d.", id)))
	case errors.Is(err, services.ErrMissingData):
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El nombre y la dirección no pueden estar vacíos."))
	default:
		c.Logs.Write(trackID, stage, "error", action+": "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo completar la operación"))
	}
}
