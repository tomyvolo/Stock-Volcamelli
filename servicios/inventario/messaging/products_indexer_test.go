package messaging

import (
	"errors"
	"testing"

	"inventario/models/products"
	productsRepo "inventario/repositories/products"
	services "inventario/services/products"
)

// newIndexer arma el indexador con los Mocks: el producto 1 está en MySQL y el índice está vacío.
func newIndexer() (ProductsIndexer, productsRepo.SearchMock) {
	search := productsRepo.SearchMock{Indexed: map[uint]products.ProductModel{}}
	service := services.ProductsService{
		Repo:   productsRepo.ProductsMock{MockDB: []products.ProductModel{{ID: 1, Name: "Harina 0000"}}},
		Search: search,
	}
	return ProductsIndexer{Service: service}, search
}

func TestIndexer_Process(t *testing.T) {
	indexer, search := newIndexer()

	trackID, err := indexer.Process([]byte(`{"id":"e-1","tipo":"ProductoActualizado","productoId":1,"trackId":"abc"}`))

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if trackID != "abc" || search.Indexed[1].Name != "Harina 0000" {
		t.Errorf("El producto no quedó en el índice: %+v", search.Indexed)
	}
}

// Un evento que no se puede leer o de un producto que no existe va directo a la cola de errores.
func TestIndexer_Discard(t *testing.T) {
	indexer, _ := newIndexer()

	for _, body := range []string{`no es json`, `{"id":"e-1"}`, `{"id":"e-1","productoId":99}`} {
		_, err := indexer.Process([]byte(body))
		if err == nil || !indexer.discard(err) {
			t.Errorf("%s: se esperaba descartarlo, pero se obtuvo: %v", body, err)
		}
	}
}

// Si Solr no responde, se reintenta: no se descarta de entrada.
func TestIndexer_SolrDownIsRetried(t *testing.T) {
	indexer, _ := newIndexer()
	indexer.Service.Search = productsRepo.SearchMock{Err: errors.New("Solr no responde")}

	_, err := indexer.Process([]byte(`{"id":"e-1","productoId":1}`))

	if err == nil || indexer.discard(err) {
		t.Errorf("Se esperaba un error que se reintente, pero se obtuvo: %v", err)
	}
}
