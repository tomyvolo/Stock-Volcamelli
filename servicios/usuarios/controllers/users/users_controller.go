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

type updateUserRequest struct {
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

// List responde GET /api/users y GET /api/users?branchId=3. Cualquier usuario puede ver a los demás, sin contraseñas.
func (c *UsersController) List(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	var branchID *uint
	if query := ctx.Query("branchId"); query != "" {
		branch, err := strconv.ParseUint(query, 10, 32)
		if err != nil || branch == 0 {
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El parámetro branchId tiene que ser un número mayor a 0."))
			return
		}
		value := uint(branch)
		branchID = &value
	}

	list, err := c.Service.ListUsers(branchID)
	if err != nil {
		c.Logs.Write(trackID, stage, "error", "listar usuarios: "+err.Error())
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudieron obtener los usuarios"))
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("listar usuarios: %d usuarios (%d ms)", len(list), time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{"items": list})
}

// Update responde PUT /api/users/:userId. El gateway solo la deja pasar con el token de un director.
// Para el director, branchId va vacío; para el admin y el vendedor, es obligatorio.
func (c *UsersController) Update(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	id, err := strconv.ParseUint(ctx.Param("userId"), 10, 32)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El userId tiene que ser un número mayor a 0."))
		return
	}
	var req updateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "Es obligatorio role, y branchId para el admin de sucursal y el vendedor."))
		return
	}

	user, err := c.Service.UpdateUser(uint(id), req.Role, req.BranchID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUserNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeUserNotFound, fmt.Sprintf("No existe el usuario %d.", id)))
		case errors.Is(err, services.ErrInvalidRole), errors.Is(err, services.ErrBranchRequired), errors.Is(err, services.ErrBranchNotAllowed):
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, err.Error()+"."))
		default:
			c.Logs.Write(trackID, stage, "error", "modificar usuario: "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo modificar el usuario"))
		}
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("usuario %d pasa a %s (%d ms)", user.ID, user.Role, time.Since(start).Milliseconds()))
	ctx.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"name":     user.Name,
		"email":    user.Email,
		"role":     user.Role,
		"branchId": user.BranchID,
	})
}

// Delete responde DELETE /api/users/:userId. El gateway solo la deja pasar con el token de un director,
// y quién lo pide llega en el header X-User-ID.
func (c *UsersController) Delete(ctx *gin.Context) {
	start := time.Now()
	trackID := ctx.GetString("track_id")

	id, err := strconv.ParseUint(ctx.Param("userId"), 10, 32)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "El userId tiene que ser un número mayor a 0."))
		return
	}
	requesterID, _ := strconv.ParseUint(ctx.GetHeader("X-User-ID"), 10, 32)

	if err := c.Service.DeleteUser(uint(requesterID), uint(id)); err != nil {
		switch {
		case errors.Is(err, services.ErrUserNotFound):
			ctx.JSON(http.StatusNotFound, utils.ErrorResponse(utils.CodeUserNotFound, fmt.Sprintf("No existe el usuario %d.", id)))
		case errors.Is(err, services.ErrDeleteSelf):
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse(utils.CodeInvalidRequest, "No podés borrar tu propio usuario."))
		default:
			c.Logs.Write(trackID, stage, "error", "borrar usuario: "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo borrar el usuario"))
		}
		return
	}

	c.Logs.Write(trackID, stage, "info", fmt.Sprintf("usuario %d borrado (%d ms)", id, time.Since(start).Milliseconds()))
	ctx.Status(http.StatusNoContent)
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
