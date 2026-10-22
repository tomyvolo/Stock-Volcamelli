package stock

import (
	"errors"
	"testing"

	"inventario/models/products"
	"inventario/models/stock"
	productsRepo "inventario/repositories/products"
	repo "inventario/repositories/stock"
)

// newService arma el Service con los dos Mocks en lugar de MySQL.
// El producto 3 existe pero no tiene stock en ninguna sucursal.
func newService() StockService {
	mockProducts := productsRepo.ProductsMock{
		MockDB: []products.ProductModel{
			{ID: 1, Name: "Harina 000"},
			{ID: 2, Name: "Azúcar"},
			{ID: 3, Name: "Yerba mate"},
		},
	}
	mockStock := repo.StockMock{
		MockDB: []stock.StockModel{
			{BranchID: 1, ProductID: 1, ProductName: "Harina 000", Quantity: 42},
			{BranchID: 1, ProductID: 2, ProductName: "Azúcar", Quantity: 5},
			{BranchID: 2, ProductID: 1, ProductName: "Harina 000", Quantity: 3},
		},
	}
	return StockService{Repo: mockStock, ProductsRepo: mockProducts}
}

func TestGetBranchStock_All(t *testing.T) {
	service := newService()

	list, err := service.GetBranchStock(1, false)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("Se esperaban 2 productos en la sucursal 1, pero se obtuvo: %+v", list)
	}
}

func TestGetBranchStock_LowStockOnly(t *testing.T) {
	service := newService()

	list, err := service.GetBranchStock(1, true)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	// El Azúcar tiene justo 5: cuenta como stock bajo. La Harina tiene 42: no.
	if len(list) != 1 || list[0].ProductID != 2 {
		t.Errorf("Se esperaba solo el producto 2, pero se obtuvo: %+v", list)
	}
}

func TestGetBranchStock_EmptyBranchIsNotNull(t *testing.T) {
	service := newService()

	list, err := service.GetBranchStock(9, false)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Errorf("Se esperaba una lista vacía, pero se obtuvo: %v", list)
	}
}

func TestGetProductStock_Success(t *testing.T) {
	service := newService()

	row, err := service.GetProductStock(2, 1)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if row.Quantity != 3 {
		t.Errorf("Se esperaban 3 unidades, pero se obtuvo: %d", row.Quantity)
	}
}

func TestGetProductStock_NoStockReturnsZero(t *testing.T) {
	service := newService()

	row, err := service.GetProductStock(1, 3)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if row.Quantity != 0 || row.ProductName != "Yerba mate" {
		t.Errorf("Se esperaba 'Yerba mate' con 0 unidades, pero se obtuvo: %+v", row)
	}
}

func TestGetProductStock_ProductNotFound(t *testing.T) {
	service := newService()

	_, err := service.GetProductStock(1, 99)

	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("Se esperaba ErrProductNotFound, pero se obtuvo: %v", err)
	}
}
