package products

// ProductModel es una fila de la tabla products. Los nombres en JSON son los del contrato.
type ProductModel struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Supplier    *string `json:"supplier"` // nil cuando no tiene proveedor
}

// TableName le dice a GORM que la tabla se llama products.
func (ProductModel) TableName() string {
	return "products"
}

// ProductListResult es una página de productos y la cantidad total de productos.
type ProductListResult struct {
	Products []ProductModel
	Total    int64
}
