package branches

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"sucursales/models/branches"
	repo "sucursales/repositories/branches"
)

var (
	ErrBranchNotFound = errors.New("la sucursal no existe")
	ErrMissingData    = errors.New("el nombre y la dirección no pueden estar vacíos")
)

type BranchesService struct {
	Repo repo.BranchesRepo
}

func (s *BranchesService) GetBranches() ([]branches.BranchModel, error) {
	return s.Repo.GetAll()
}

func (s *BranchesService) GetBranch(id uint) (branches.BranchModel, error) {
	branch, err := s.Repo.GetByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return branches.BranchModel{}, ErrBranchNotFound
	}
	return branch, err
}

// CreateBranch da de alta una sucursal. Que lo pida un director lo controla el gateway.
func (s *BranchesService) CreateBranch(name, address string) (branches.BranchModel, error) {
	branch := branches.BranchModel{Name: strings.TrimSpace(name), Address: strings.TrimSpace(address)}
	if branch.Name == "" || branch.Address == "" {
		return branches.BranchModel{}, ErrMissingData
	}
	if err := s.Repo.Create(&branch); err != nil {
		return branches.BranchModel{}, err
	}
	return branch, nil
}

// UpdateBranch cambia el nombre y la dirección, por ejemplo cuando la sucursal se muda.
func (s *BranchesService) UpdateBranch(id uint, name, address string) (branches.BranchModel, error) {
	branch := branches.BranchModel{ID: id, Name: strings.TrimSpace(name), Address: strings.TrimSpace(address)}
	if branch.Name == "" || branch.Address == "" {
		return branches.BranchModel{}, ErrMissingData
	}
	if _, err := s.GetBranch(id); err != nil {
		return branches.BranchModel{}, err
	}
	if err := s.Repo.Update(branch); err != nil {
		return branches.BranchModel{}, err
	}
	return branch, nil
}
