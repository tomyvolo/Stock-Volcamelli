package branches

// BranchModel es una fila de la tabla branches. Se guarda igual en Memcached, como JSON.
type BranchModel struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (BranchModel) TableName() string {
	return "branches"
}
