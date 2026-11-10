package users

import (
	"sort"

	"gorm.io/gorm"

	"usuarios/models/users"
)

// UsersMock simula ser MySQL, pero vive solo en la RAM. La clave del mapa es el email.
// Calls cuenta cuántas veces se pidió la lista, para comprobar en los tests si la caché respondió antes.
type UsersMock struct {
	MockDB map[string]users.UserModel
	Calls  *int
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

// GetUserByID recorre el mapa buscando el id, porque la clave es el email.
func (repo UsersMock) GetUserByID(id uint) (users.UserModel, error) {
	for _, user := range repo.MockDB {
		if user.ID == id {
			return user, nil
		}
	}
	return users.UserModel{}, gorm.ErrRecordNotFound
}

func (repo UsersMock) UpdateRoleAndBranch(id uint, role string, branchID *uint) error {
	user, err := repo.GetUserByID(id)
	if err != nil {
		return err
	}
	user.Role = role
	user.BranchID = branchID
	repo.MockDB[user.Email] = user
	return nil
}

func (repo UsersMock) DeleteUser(id uint) error {
	user, err := repo.GetUserByID(id)
	if err != nil {
		return err
	}
	delete(repo.MockDB, user.Email)
	return nil
}

// GetAll devuelve los usuarios ordenados por id y sin la contraseña, como MySQL.
func (repo UsersMock) GetAll() ([]users.UserSummary, error) {
	if repo.Calls != nil {
		*repo.Calls++
	}
	list := make([]users.UserSummary, 0, len(repo.MockDB))
	for _, user := range repo.MockDB {
		list = append(list, users.UserSummary{ID: user.ID, Name: user.Name, Email: user.Email, Role: user.Role, BranchID: user.BranchID})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}
