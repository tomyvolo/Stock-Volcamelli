package products

import (
	"errors"

	"gorm.io/gorm"

	"inventario/models/products"
	repo "inventario/repositories/products"
)

var ErrProductNotFound = errors.New("producto no encontrado")

type ProductsService struct {
	// Inyectamos la interfaz, NO la base de datos real
	Repo repo.ProductsRepo
}

func (s *ProductsService) GetProducts(page, size int) (products.ProductListResult, error) {
	result, err := s.Repo.GetProducts(page, size)
	if err != nil {
		return products.ProductListResult{}, err
	}
	// Una página sin productos se responde como lista vacía, no como null.
	if result.Products == nil {
		result.Products = []products.ProductModel{}
	}
	return result, nil
}

func (s *ProductsService) GetProduct(id uint) (products.ProductModel, error) {
	product, err := s.Repo.GetProductByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return products.ProductModel{}, ErrProductNotFound
	}
	return product, err
}
