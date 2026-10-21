package users

import "usuarios/models/users"

type UsersRepo interface {
	GetUserByEmail(email string) (users.UserModel, error)
}
