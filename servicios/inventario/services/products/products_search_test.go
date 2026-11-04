package products

import (
	"errors"
	"testing"

	"inventario/models/products"
	repo "inventario/repositories/products"
)

// newSearchService arma el Service con 3 productos en MySQL y el índice vacío.
func newSearchService() (ProductsService, repo.SearchMock) {
	mock := repo.ProductsMock{MockDB: []products.ProductModel{
		{ID: 1, Name: "Harina 000"}, {ID: 2, Name: "Harina integral"}, {ID: 3, Name: "Azúcar"},
	}}
	search := repo.SearchMock{Indexed: map[uint]products.ProductModel{}}
	return ProductsService{Repo: mock, Search: search}, search
}

func TestSearchProducts_NoResults(t *testing.T) {
	service, _ := newSearchService()

	result, err := service.SearchProducts(products.SearchQuery{Text: "yerba", Page: 1, Size: 20})

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	// Sin resultados se responde una lista vacía, no null.
	if result.Results == nil || result.Total != 0 {
		t.Errorf("Se esperaba una lista vacía, pero se obtuvo: %+v", result)
	}
}

// CA-27: si Solr no responde, la búsqueda devuelve el error, no una lista vacía.
func TestSearchProducts_SolrDown(t *testing.T) {
	service, _ := newSearchService()
	service.Search = repo.SearchMock{Err: errors.New("Solr no responde")}

	_, err := service.SearchProducts(products.SearchQuery{Text: "harina", Page: 1, Size: 20})

	if err == nil {
		t.Errorf("Se esperaba un error")
	}
}

func TestRebuildIndex_WritesAllProducts(t *testing.T) {
	service, search := newSearchService()

	written, err := service.RebuildIndex()

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if written != 3 || len(search.Indexed) != 3 {
		t.Errorf("Se esperaban 3 productos en el índice, pero se escribieron %d", written)
	}
	result, _ := service.SearchProducts(products.SearchQuery{Text: "harina", Page: 1, Size: 20})
	if result.Total != 2 {
		t.Errorf("Se esperaban 2 harinas, pero se encontraron %d", result.Total)
	}
}

// CA-26: después de cambiarle el nombre, la búsqueda lo encuentra con el nombre nuevo.
func TestIndexProduct_ReplacesDocument(t *testing.T) {
	service, search := newSearchService()
	service.RebuildIndex()
	service.Repo.UpdateProduct(products.ProductModel{ID: 1, Name: "Harina 0000"})

	if err := service.IndexProduct(1); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if search.Indexed[1].Name != "Harina 0000" {
		t.Errorf("Se esperaba el nombre nuevo en el índice, pero quedó: %q", search.Indexed[1].Name)
	}
}

func TestIndexProduct_NotFound(t *testing.T) {
	service, _ := newSearchService()

	err := service.IndexProduct(99)

	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("Se esperaba ErrProductNotFound, pero se obtuvo: %v", err)
	}
}
