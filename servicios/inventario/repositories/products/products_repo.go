package products

import "inventario/models/products"

type ProductsRepo interface {
	GetProducts(page, size int) (products.ProductListResult, error)
	GetProductByID(id uint) (products.ProductModel, error)
	CreateProduct(product *products.ProductModel) error
	UpdateProduct(product products.ProductModel) error
}
