package stock

// LowStockLimit es la cantidad disponible desde la que un producto tiene stock bajo.
const LowStockLimit = 5

// StockRow es una fila de la tabla stock, con el total y lo reservado de un producto en una sucursal.
// La usan las operaciones que cambian el stock; las consultas devuelven StockModel.
type StockRow struct {
	ProductID uint `gorm:"primaryKey"`
	BranchID  uint `gorm:"primaryKey"`
	Total     int
	Reserved  int
}

func (StockRow) TableName() string {
	return "stock"
}

// StockModel es el stock disponible de un producto en una sucursal: total - reserved.
// Los nombres en JSON son los del contrato.
type StockModel struct {
	BranchID    uint   `json:"branchId"`
	ProductID   uint   `json:"productId"`
	ProductName string `json:"productName"`
	Quantity    int    `json:"quantity"`
}
