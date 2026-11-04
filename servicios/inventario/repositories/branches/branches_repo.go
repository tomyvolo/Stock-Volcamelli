package branches

// BranchesRepo consulta a Sucursales, que es el dueño de las sucursales. Inventario solo guarda su id.
type BranchesRepo interface {
	// Exists devuelve false si Sucursales responde que no existe, y error si no responde.
	Exists(branchID uint, trackID string) (bool, error)
}
