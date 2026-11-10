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

// GetUserByID devuelve gorm.ErrRecordNotFound si no hay un usuario con ese id.
func (repo UsersMySQL) GetUserByID(id uint) (users.UserModel, error) {
	var user users.UserModel
	err := repo.DB.First(&user, id).Error
	return user, err
}

// UpdateRoleAndBranch cambia solo el rol y la sucursal. Con un map, GORM también guarda la sucursal en NULL.
func (repo UsersMySQL) UpdateRoleAndBranch(id uint, role string, branchID *uint) error {
	return repo.DB.Model(&users.UserModel{}).Where("id = ?", id).
		Updates(map[string]any{"role": role, "branch_id": branchID}).Error
}

func (repo UsersMySQL) DeleteUser(id uint) error {
	return repo.DB.Delete(&users.UserModel{}, id).Error
}

// GetAll devuelve todos los usuarios ordenados por id. No lee la columna de la contraseña.
func (repo UsersMySQL) GetAll() ([]users.UserSummary, error) {
	var list []users.UserSummary
	err := repo.DB.Model(&users.UserModel{}).Select("id", "name", "email", "role", "branch_id").Order("id").Find(&list).Error
	return list, err
}
