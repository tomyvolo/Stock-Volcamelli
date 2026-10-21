package users

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"usuarios/logs"
	services "usuarios/services/users"
	"usuarios/utils"
)

const stage = "usuarios"

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UsersController struct {
	Service services.UsersService
	Logs    *logs.Store
}

// Login responde POST /api/auth/login. Nunca escribe la contraseña en los logs.
func (c *UsersController) Login(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Faltan el email o la contraseña, o el email no es válido."))
		return
	}

	token, user, err := c.Service.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			c.Logs.Write(trackID, stage, "warn", "login rechazado: email o contraseña incorrectos")
			ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse(utils.CodeInvalidCredentials, "El email o la contraseña son incorrectos."))
			return
		}
		c.Logs.Write(trackID, stage, "error", "login: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo iniciar sesión"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("login correcto del usuario %d con rol %s (%d ms)",
		user.ID, user.Role, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresIn": utils.TokenHours() * 3600,
	})
}
