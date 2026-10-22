package stock

import (
	"gorm.io/gorm"

	"inventario/models/stock"
)

type StockMySQL struct {
	DB *gorm.DB
}

// query arma el SELECT del stock disponible junto con el nombre del producto.
// El CHECK de la tabla asegura que reserved nunca supere a total.
func (repo StockMySQL) query() *gorm.DB {
	return repo.DB.Table("stock AS s").
		Select("s.branch_id, s.product_id, p.name AS product_name, s.total - s.reserved AS quantity").
		Joins("JOIN products p ON p.id = s.product_id")
}

func (repo StockMySQL) GetStockByBranch(branchID uint, lowStock bool) ([]stock.StockModel, error) {
	var list []stock.StockModel

	query := repo.query().Where("s.branch_id = ?", branchID)
	if lowStock {
		query = query.Where("s.total - s.reserved <= ?", stock.LowStockLimit)
	}

	err := query.Order("p.name ASC").Scan(&list).Error
	return list, err
}

// GetStockByProduct devuelve gorm.ErrRecordNotFound si el producto no tiene stock cargado en la sucursal.
func (repo StockMySQL) GetStockByProduct(branchID, productID uint) (stock.StockModel, error) {
	var row stock.StockModel
	err := repo.query().
		Where("s.branch_id = ? AND s.product_id = ?", branchID, productID).
		Take(&row).Error
	return row, err
}
