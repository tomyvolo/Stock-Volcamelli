package stock

import (
	"gorm.io/gorm"

	"inventario/models/stock"
)

// StockMock simula ser MySQL, pero vive solo en la RAM.
// Cada elemento de MockDB es el stock disponible de un producto en una sucursal.
type StockMock struct {
	MockDB []stock.StockModel
}

func (repo StockMock) GetStockByBranch(branchID uint, lowStock bool) ([]stock.StockModel, error) {
	var list []stock.StockModel
	for _, row := range repo.MockDB {
		if row.BranchID != branchID {
			continue
		}
		if lowStock && row.Quantity > stock.LowStockLimit {
			continue
		}
		list = append(list, row)
	}
	return list, nil
}

// GetStockByProduct devuelve el mismo error que GORM cuando no hay stock cargado.
func (repo StockMock) GetStockByProduct(branchID, productID uint) (stock.StockModel, error) {
	for _, row := range repo.MockDB {
		if row.BranchID == branchID && row.ProductID == productID {
			return row, nil
		}
	}
	return stock.StockModel{}, gorm.ErrRecordNotFound
}
