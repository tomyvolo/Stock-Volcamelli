package branches

import "slices"

// BranchesList es la lista fija de sucursales de Inventario publicado, donde no está Sucursales.
type BranchesList struct {
	IDs []uint
}

func (repo BranchesList) Exists(branchID uint, trackID string) (bool, error) {
	return slices.Contains(repo.IDs, branchID), nil
}
