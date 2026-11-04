package products

import "inventario/models/products"

// SearchRepo es el buscador. El service lo usa sin saber que atrás está Solr.
type SearchRepo interface {
	Search(query products.SearchQuery) (products.SearchResultList, error)
	// SaveProducts reemplaza el documento entero de cada producto, o lo crea si no estaba.
	SaveProducts(list []products.ProductModel) error
	// CreateFields crea los campos del índice si todavía no existen.
	CreateFields() error
}
