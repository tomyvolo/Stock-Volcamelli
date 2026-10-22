package stock

import "inventario/models/stock"

type StockRepo interface {
	// GetStockByBranch devuelve el stock disponible de cada producto de la sucursal.
	// Con lowStock, solo los que tienen stock.LowStockLimit unidades o menos.
	GetStockByBranch(branchID uint, lowStock bool) ([]stock.StockModel, error)
	GetStockByProduct(branchID, productID uint) (stock.StockModel, error)
}
