package users

// UserModel es una fila de la tabla users.
type UserModel struct {
	ID           uint
	Name         string
	Email        string
	PasswordHash string
	Role         string
	BranchID     *uint // nil para DIRECTOR e INTEGRACION
}

// TableName le dice a GORM que la tabla se llama users.
func (UserModel) TableName() string {
	return "users"
}
