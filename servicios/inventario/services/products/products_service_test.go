package products

import (
	"errors"
	"testing"

	"inventario/models/products"
	repo "inventario/repositories/products"
)

// newService arma el Service con el Mock en lugar de MySQL, con 3 productos cargados.
func newService() ProductsService {
	supplier := "Molinos del Sur"
	mockRepo := repo.ProductsMock{
		MockDB: []products.ProductModel{
			{ID: 1, Name: "Harina 000", Description: "Paquete de 1 kg", Supplier: &supplier},
			{ID: 2, Name: "Azúcar", Description: "Paquete de 1 kg"},
			{ID: 3, Name: "Yerba mate", Description: "Paquete de 1 kg"},
		},
	}
	return ProductsService{Repo: mockRepo}
}

func TestGetProducts_Pagination(t *testing.T) {
	service := newService()

	result, err := service.GetProducts(2, 2)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if len(result.Products) != 1 || result.Products[0].ID != 3 {
		t.Errorf("La página 2 de a 2 tenía que traer solo el producto 3, pero trajo: %v", result.Products)
	}
	if result.Total != 3 {
		t.Errorf("Se esperaba total 3, pero se obtuvo: %d", result.Total)
	}
}

func TestGetProducts_EmptyPageIsNotNull(t *testing.T) {
	service := newService()

	result, err := service.GetProducts(5, 20)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if result.Products == nil || len(result.Products) != 0 {
		t.Errorf("Se esperaba una lista vacía, pero se obtuvo: %v", result.Products)
	}
}

func TestGetProduct_Success(t *testing.T) {
	service := newService()

	product, err := service.GetProduct(1)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if product.Name != "Harina 000" || product.Supplier == nil {
		t.Errorf("Se esperaba 'Harina 000' con proveedor, pero se obtuvo: %+v", product)
	}
}

func TestGetProduct_NotFound(t *testing.T) {
	service := newService()

	_, err := service.GetProduct(99)

	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("Se esperaba ErrProductNotFound, pero se obtuvo: %v", err)
	}
}
