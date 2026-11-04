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

// CreateProduct inserta el producto y GORM le completa el ID que asignó MySQL.
func (repo ProductsMySQL) CreateProduct(product *products.ProductModel) error {
	return repo.DB.Create(product).Error
}

// UpdateProduct guarda todos los campos de un producto que ya existe, incluido el proveedor en NULL.
func (repo ProductsMySQL) UpdateProduct(product products.ProductModel) error {
	return repo.DB.Save(&product).Error
}

// GetProductByID devuelve gorm.ErrRecordNotFound si el producto no existe.
func (repo ProductsMySQL) GetProductByID(id uint) (products.ProductModel, error) {
	var product products.ProductModel
	err := repo.DB.First(&product, id).Error
	return product, err
}
