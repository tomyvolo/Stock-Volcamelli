package users

import (
	"gorm.io/gorm"

	"usuarios/models/users"
)

type UsersMySQL struct {
	DB *gorm.DB
}

// GetUserByEmail devuelve gorm.ErrRecordNotFound si no hay un usuario con ese email.
func (repo UsersMySQL) GetUserByEmail(email string) (users.UserModel, error) {
	var user users.UserModel
	err := repo.DB.Where("email = ?", email).First(&user).Error
	return user, err
}

// CreateUser inserta el usuario y GORM le completa el ID que asignó MySQL.
func (repo UsersMySQL) CreateUser(user *users.UserModel) error {
	return repo.DB.Create(user).Error
}
