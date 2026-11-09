package branches

import "sucursales/models/branches"

type BranchesRepo interface {
	GetAll() ([]branches.BranchModel, error)
	GetByID(id uint) (branches.BranchModel, error)
	Create(branch *branches.BranchModel) error
	Update(branch branches.BranchModel) error
}
