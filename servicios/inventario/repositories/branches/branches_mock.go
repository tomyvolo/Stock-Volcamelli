package branches

// BranchesMock simula a Sucursales. Con Err distinto de nil, simula que Sucursales no responde.
type BranchesMock struct {
	Existing map[uint]bool
	Err      error
}

func (repo BranchesMock) Exists(branchID uint, trackID string) (bool, error) {
	if repo.Err != nil {
		return false, repo.Err
	}
	return repo.Existing[branchID], nil
}
