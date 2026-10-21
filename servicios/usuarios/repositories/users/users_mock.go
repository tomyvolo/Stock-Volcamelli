package users

import (
	"gorm.io/gorm"

	"usuarios/models/users"
)

// UsersMock simula ser MySQL, pero vive solo en la RAM. La clave del mapa es el email.
type UsersMock struct {
	MockDB map[string]users.UserModel
}

// GetUserByEmail devuelve el mismo error que GORM cuando el usuario no existe.
func (repo UsersMock) GetUserByEmail(email string) (users.UserModel, error) {
	user, exists := repo.MockDB[email]
	if !exists {
		return users.UserModel{}, gorm.ErrRecordNotFound
	}
	return user, nil
}
