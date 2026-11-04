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

// CreateUser guarda el usuario en el mapa y le asigna el siguiente ID, como haría MySQL.
func (repo UsersMock) CreateUser(user *users.UserModel) error {
	user.ID = uint(len(repo.MockDB) + 1)
	repo.MockDB[user.Email] = *user
	return nil
}
