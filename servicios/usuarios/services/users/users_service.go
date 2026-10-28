package users

import (
	"errors"

	"gorm.io/gorm"

	"usuarios/models/users"
	repo "usuarios/repositories/users"
	"usuarios/utils"
)

var (
	ErrInvalidCredentials = errors.New("credenciales inválidas")
	ErrInvalidRole        = errors.New("el rol no existe")
	ErrBranchRequired     = errors.New("el admin de sucursal y el vendedor necesitan una sucursal")
	ErrBranchNotAllowed   = errors.New("el director no lleva sucursal")
	ErrNotAllowed         = errors.New("no tiene permiso para crear este usuario")
	ErrEmailTaken         = errors.New("ya existe un usuario con ese email")
)

// Roles que puede tener un usuario.
const (
	RoleDirector      = "DIRECTOR"
	RoleAdminSucursal = "ADMIN_SUCURSAL"
	RoleVendedor      = "VENDEDOR"
)

type UsersService struct {
	Repo repo.UsersRepo
}

// Login compara la contraseña con el hash guardado y, si coincide, devuelve el token y el usuario.
// Si el email no existe o la contraseña no coincide, devuelve el mismo error, para no dar pistas.
func (s *UsersService) Login(email, password string) (string, users.UserModel, error) {
	user, err := s.Repo.GetUserByEmail(email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", users.UserModel{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", users.UserModel{}, err
	}

	if !utils.CheckPassword(password, user.PasswordHash) {
		return "", users.UserModel{}, ErrInvalidCredentials
	}

	token, err := utils.GenerateJWT(user.ID, user.Email, user.Role, user.BranchID)
	if err != nil {
		return "", users.UserModel{}, err
	}

	return token, user, nil
}

// CreateUser da de alta un usuario. creatorRole y creatorBranch son los de quien lo pide, sacados de su token.
// El director crea usuarios de cualquier rol; el admin de sucursal, solo vendedores de su sucursal.
func (s *UsersService) CreateUser(creatorRole string, creatorBranch *uint, user users.UserModel, password string) (users.UserModel, error) {
	switch user.Role {
	case RoleAdminSucursal, RoleVendedor:
		if user.BranchID == nil {
			return users.UserModel{}, ErrBranchRequired
		}
	case RoleDirector:
		if user.BranchID != nil {
			return users.UserModel{}, ErrBranchNotAllowed
		}
	default:
		return users.UserModel{}, ErrInvalidRole
	}

	switch creatorRole {
	case RoleDirector:
	case RoleAdminSucursal:
		if user.Role != RoleVendedor || creatorBranch == nil || *user.BranchID != *creatorBranch {
			return users.UserModel{}, ErrNotAllowed
		}
	default:
		return users.UserModel{}, ErrNotAllowed
	}

	_, err := s.Repo.GetUserByEmail(user.Email)
	if err == nil {
		return users.UserModel{}, ErrEmailTaken
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return users.UserModel{}, err
	}

	user.PasswordHash, err = utils.HashPassword(password)
	if err != nil {
		return users.UserModel{}, err
	}
	if err := s.Repo.CreateUser(&user); err != nil {
		return users.UserModel{}, err
	}
	return user, nil
}
