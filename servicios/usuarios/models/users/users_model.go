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

// UserSummary es lo que cualquier usuario puede ver de los demás. No tiene la contraseña,
// así que tampoco llega a Memcached.
type UserSummary struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	BranchID *uint  `json:"branchId"`
}
