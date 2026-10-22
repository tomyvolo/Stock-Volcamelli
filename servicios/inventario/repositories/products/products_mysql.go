package products

import (
	"gorm.io/gorm"

	"inventario/models/products"
)

type ProductsMySQL struct {
	DB *gorm.DB
}

func (repo ProductsMySQL) GetProducts(page, size int) (products.ProductListResult, error) {
	var list []products.ProductModel
	var total int64

	if err := repo.DB.Model(&products.ProductModel{}).Count(&total).Error; err != nil {
		return products.ProductListResult{}, err
	}

	offset := (page - 1) * size
	if err := repo.DB.Order("id ASC").Limit(size).Offset(offset).Find(&list).Error; err != nil {
		return products.ProductListResult{}, err
	}

	return products.ProductListResult{Products: list, Total: total}, nil
}

// GetProductByID devuelve gorm.ErrRecordNotFound si el producto no existe.
func (repo ProductsMySQL) GetProductByID(id uint) (products.ProductModel, error) {
	var product products.ProductModel
	err := repo.DB.First(&product, id).Error
	return product, err
}
