package products

import (
	"gorm.io/gorm"

	"inventario/models/products"
)

// ProductsMock simula ser MySQL, pero vive solo en la RAM.
// MockDB tiene que estar ordenado por id, como lo devuelve MySQL.
type ProductsMock struct {
	MockDB []products.ProductModel
}

func (repo ProductsMock) GetProducts(page, size int) (products.ProductListResult, error) {
	total := int64(len(repo.MockDB))
	offset := (page - 1) * size
	if offset >= len(repo.MockDB) {
		return products.ProductListResult{Total: total}, nil
	}
	end := min(offset+size, len(repo.MockDB))
	return products.ProductListResult{Products: repo.MockDB[offset:end], Total: total}, nil
}

// GetProductByID devuelve el mismo error que GORM cuando el producto no existe.
func (repo ProductsMock) GetProductByID(id uint) (products.ProductModel, error) {
	for _, product := range repo.MockDB {
		if product.ID == id {
			return product, nil
		}
	}
	return products.ProductModel{}, gorm.ErrRecordNotFound
}
