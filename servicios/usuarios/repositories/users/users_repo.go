package users

import "usuarios/models/users"

type UsersRepo interface {
	GetUserByEmail(email string) (users.UserModel, error)
	CreateUser(user *users.UserModel) error
	GetAll() ([]users.UserSummary, error)
	GetUserByID(id uint) (users.UserModel, error)
	UpdateRoleAndBranch(id uint, role string, branchID *uint) error
	DeleteUser(id uint) error
}
