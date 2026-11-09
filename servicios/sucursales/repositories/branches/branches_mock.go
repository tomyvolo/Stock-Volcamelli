package branches

import (
	"sort"

	"gorm.io/gorm"

	"sucursales/models/branches"
)

// BranchesMock simula ser MySQL, pero vive solo en la RAM. La clave del mapa es el id.
// Calls cuenta cuántas consultas le llegaron, para comprobar en los tests si la caché respondió antes.
type BranchesMock struct {
	MockDB map[uint]branches.BranchModel
	Calls  *int
}

func (repo BranchesMock) count() {
	if repo.Calls != nil {
		*repo.Calls++
	}
}

// GetAll devuelve las sucursales ordenadas por id, como MySQL.
func (repo BranchesMock) GetAll() ([]branches.BranchModel, error) {
	repo.count()
	list := make([]branches.BranchModel, 0, len(repo.MockDB))
	for _, branch := range repo.MockDB {
		list = append(list, branch)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

// GetByID devuelve el mismo error que GORM cuando la sucursal no existe.
func (repo BranchesMock) GetByID(id uint) (branches.BranchModel, error) {
	repo.count()
	branch, exists := repo.MockDB[id]
	if !exists {
		return branches.BranchModel{}, gorm.ErrRecordNotFound
	}
	return branch, nil
}

// Create guarda la sucursal en el mapa y le asigna el siguiente ID, como haría MySQL.
func (repo BranchesMock) Create(branch *branches.BranchModel) error {
	branch.ID = uint(len(repo.MockDB) + 1)
	repo.MockDB[branch.ID] = *branch
	return nil
}

func (repo BranchesMock) Update(branch branches.BranchModel) error {
	repo.MockDB[branch.ID] = branch
	return nil
}
