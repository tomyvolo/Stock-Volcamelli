package users

import (
	"errors"

	"gorm.io/gorm"

	"usuarios/models/users"
	repo "usuarios/repositories/users"
	"usuarios/utils"
)

var ErrInvalidCredentials = errors.New("credenciales inválidas")

type UsersService struct {
	// Inyectamos la interfaz, NO la base de datos real
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
