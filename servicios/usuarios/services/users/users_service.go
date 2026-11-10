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
	ErrUserNotFound       = errors.New("el usuario no existe")
	ErrDeleteSelf         = errors.New("no se puede borrar el propio usuario")
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

// ListUsers devuelve todos los usuarios de la empresa, sin contraseñas. Con branchID, solo los de esa sucursal.
// El filtro se hace sobre la lista completa, que es la que queda en la caché.
func (s *UsersService) ListUsers(branchID *uint) ([]users.UserSummary, error) {
	list, err := s.Repo.GetAll()
	if err != nil || branchID == nil {
		return list, err
	}
	filtered := []users.UserSummary{}
	for _, user := range list {
		if user.BranchID != nil && *user.BranchID == *branchID {
			filtered = append(filtered, user)
		}
	}
	return filtered, nil
}

// checkRoleAndBranch controla que el rol exista y que la sucursal vaya solo cuando corresponde.
func checkRoleAndBranch(role string, branchID *uint) error {
	switch role {
	case RoleAdminSucursal, RoleVendedor:
		if branchID == nil {
			return ErrBranchRequired
		}
	case RoleDirector:
		if branchID != nil {
			return ErrBranchNotAllowed
		}
	default:
		return ErrInvalidRole
	}
	return nil
}

// UpdateUser cambia el rol y la sucursal de un usuario. Que lo pida un director lo controla el gateway.
// El cambio se aplica cuando el usuario vuelve a hacer login, porque el rol y la sucursal van en su token.
func (s *UsersService) UpdateUser(id uint, role string, branchID *uint) (users.UserModel, error) {
	if err := checkRoleAndBranch(role, branchID); err != nil {
		return users.UserModel{}, err
	}
	user, err := s.Repo.GetUserByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return users.UserModel{}, ErrUserNotFound
	}
	if err != nil {
		return users.UserModel{}, err
	}
	if err := s.Repo.UpdateRoleAndBranch(id, role, branchID); err != nil {
		return users.UserModel{}, err
	}
	user.Role = role
	user.BranchID = branchID
	return user, nil
}

// DeleteUser borra un usuario. Que lo pida un director lo controla el gateway. Nadie puede borrarse a sí
// mismo: así siempre queda al menos un director.
func (s *UsersService) DeleteUser(requesterID, id uint) error {
	if requesterID == id {
		return ErrDeleteSelf
	}
	if _, err := s.Repo.GetUserByID(id); errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	} else if err != nil {
		return err
	}
	return s.Repo.DeleteUser(id)
}

// CreateUser da de alta un usuario. creatorRole y creatorBranch son los de quien lo pide, sacados de su token.
// El director crea usuarios de cualquier rol; el admin de sucursal, solo vendedores de su sucursal.
func (s *UsersService) CreateUser(creatorRole string, creatorBranch *uint, user users.UserModel, password string) (users.UserModel, error) {
	if err := checkRoleAndBranch(user.Role, user.BranchID); err != nil {
		return users.UserModel{}, err
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
