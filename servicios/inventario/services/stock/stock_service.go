package stock

import (
	"errors"

	"gorm.io/gorm"

	"inventario/models/stock"
	productsRepo "inventario/repositories/products"
	repo "inventario/repositories/stock"
)

var ErrProductNotFound = errors.New("producto no encontrado")

type StockService struct {
	// Inyectamos las interfaces, NO la base de datos real
	Repo repo.StockRepo
	// Lo usamos para saber si un producto sin stock en la sucursal existe o no.
	ProductsRepo productsRepo.ProductsRepo
}

func (s *StockService) GetBranchStock(branchID uint, lowStock bool) ([]stock.StockModel, error) {
	list, err := s.Repo.GetStockByBranch(branchID, lowStock)
	if err != nil {
		return nil, err
	}
	// Una sucursal sin productos se responde como lista vacía, no como null.
	if list == nil {
		list = []stock.StockModel{}
	}
	return list, nil
}

// GetProductStock devuelve el stock disponible de un producto en una sucursal.
// Si el producto existe pero no tiene stock cargado ahí, la cantidad es 0.
func (s *StockService) GetProductStock(branchID, productID uint) (stock.StockModel, error) {
	row, err := s.Repo.GetStockByProduct(branchID, productID)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return stock.StockModel{}, err
	}

	product, err := s.ProductsRepo.GetProductByID(productID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return stock.StockModel{}, ErrProductNotFound
	}
	if err != nil {
		return stock.StockModel{}, err
	}
	return stock.StockModel{BranchID: branchID, ProductID: productID, ProductName: product.Name, Quantity: 0}, nil
}
