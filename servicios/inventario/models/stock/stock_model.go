package stock

// LowStockLimit es la cantidad disponible desde la que un producto tiene stock bajo.
const LowStockLimit = 5

// StockModel es el stock disponible de un producto en una sucursal: total - reserved.
// Los nombres en JSON son los del contrato.
type StockModel struct {
	BranchID    uint   `json:"branchId"`
	ProductID   uint   `json:"productId"`
	ProductName string `json:"productName"`
	Quantity    int    `json:"quantity"`
}
