package products

import (
	"strings"

	"inventario/models/products"
)

// SearchMock simula ser Solr: Indexed tiene los productos guardados, por id.
// Busca el texto en el nombre, sin relevancia. Si Err no es nil, todas las operaciones fallan con ese error.
type SearchMock struct {
	Indexed map[uint]products.ProductModel
	Err     error
}

func (repo SearchMock) Search(query products.SearchQuery) (products.SearchResultList, error) {
	if repo.Err != nil {
		return products.SearchResultList{}, repo.Err
	}
	var results []products.SearchResult
	for _, product := range repo.Indexed {
		if strings.Contains(strings.ToLower(product.Name), strings.ToLower(query.Text)) {
			results = append(results, products.SearchResult{ID: product.ID, Name: product.Name, Supplier: product.Supplier})
		}
	}
	return products.SearchResultList{Results: results, Total: int64(len(results))}, nil
}

func (repo SearchMock) SaveProducts(list []products.ProductModel) error {
	if repo.Err != nil {
		return repo.Err
	}
	for _, product := range list {
		repo.Indexed[product.ID] = product
	}
	return nil
}

func (repo SearchMock) CreateFields() error {
	return repo.Err
}
