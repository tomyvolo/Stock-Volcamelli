package users

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"usuarios/logs"
	"usuarios/models/users"
	services "usuarios/services/users"
	"usuarios/utils"
)

const stage = "usuarios"

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type createUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required"`
	BranchID *uint  `json:"branchId"`
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

// Create responde POST /api/create-users. Quién lo pide (rol y sucursal) llega del gateway en los headers X-User-*.
// Nunca escribe la contraseña en los logs ni la devuelve.
func (c *UsersController) Create(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Son obligatorios name, email, password y role. El email tiene que ser válido y la contraseña de al menos 6 caracteres."))
		return
	}

	creatorRole := ctx.GetHeader("X-User-Role")
	var creatorBranch *uint
	if header := ctx.GetHeader("X-User-Branch"); header != "" {
		branch, err := strconv.ParseUint(header, 10, 32)
		if err == nil {
			value := uint(branch)
			creatorBranch = &value
		}
	}

	newUser := users.UserModel{Name: req.Name, Email: req.Email, Role: req.Role, BranchID: req.BranchID}
	user, err := c.Service.CreateUser(creatorRole, creatorBranch, newUser, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrNotAllowed):
			c.Logs.Write(trackID, stage, "warn", fmt.Sprintf("alta de usuario rechazada: un %s no puede crear un %s", creatorRole, req.Role))
			ctx.JSON(http.StatusForbidden, utils.ErrorResponse(utils.CodeForbidden, "No tenés permiso para crear este usuario."))
		case errors.Is(err, services.ErrEmailTaken):
			ctx.JSON(http.StatusConflict, utils.ErrorResponse(utils.CodeEmailTaken, "Ya existe un usuario con ese email."))
		case errors.Is(err, services.ErrInvalidRole), errors.Is(err, services.ErrBranchRequired), errors.Is(err, services.ErrBranchNotAllowed):
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, err.Error()+"."))
		default:
			c.Logs.Write(trackID, stage, "error", "alta de usuario: "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo crear el usuario"))
		}
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("usuario %d creado con rol %s por un %s (%d ms)",
		user.ID, user.Role, creatorRole, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"name":     user.Name,
		"email":    user.Email,
		"role":     user.Role,
		"branchId": user.BranchID,
	})
}
