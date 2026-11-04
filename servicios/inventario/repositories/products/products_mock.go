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

// CreateProduct no puede agregar al slice del mock (es una copia), así que solo le asigna el ID.
// Los tests de alta comprueban el producto que devuelve el service.
func (repo ProductsMock) CreateProduct(product *products.ProductModel) error {
	product.ID = uint(len(repo.MockDB) + 1)
	return nil
}

func (repo ProductsMock) UpdateProduct(product products.ProductModel) error {
	for i := range repo.MockDB {
		if repo.MockDB[i].ID == product.ID {
			repo.MockDB[i] = product
			return nil
		}
	}
	return gorm.ErrRecordNotFound
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
