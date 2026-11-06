package branches

import (
	"gorm.io/gorm"

	"sucursales/models/branches"
)

type BranchesMySQL struct {
	DB *gorm.DB
}

// GetAll devuelve las sucursales ordenadas por id.
func (repo BranchesMySQL) GetAll() ([]branches.BranchModel, error) {
	var list []branches.BranchModel
	err := repo.DB.Order("id").Find(&list).Error
	return list, err
}

// GetByID devuelve gorm.ErrRecordNotFound si la sucursal no existe.
func (repo BranchesMySQL) GetByID(id uint) (branches.BranchModel, error) {
	var branch branches.BranchModel
	err := repo.DB.First(&branch, id).Error
	return branch, err
}

// Create inserta la sucursal y GORM le completa el ID que asignó MySQL.
func (repo BranchesMySQL) Create(branch *branches.BranchModel) error {
	return repo.DB.Create(branch).Error
}

// Update guarda el nombre y la dirección de una sucursal que ya existe.
func (repo BranchesMySQL) Update(branch branches.BranchModel) error {
	return repo.DB.Save(&branch).Error
}
