package users

// UserModel es una fila de la tabla users.
type UserModel struct {
	ID           uint
	Name         string
	Email        string
	PasswordHash string
	Role         string
	BranchID     *uint // nil para DIRECTOR
}

func (UserModel) TableName() string {
	return "users"
}
