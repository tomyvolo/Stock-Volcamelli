package stock

import (
	"errors"
	"testing"

	"inventario/models/movements"
	"inventario/models/products"
	"inventario/models/reservations"
	"inventario/models/stock"
	branchesRepo "inventario/repositories/branches"
	productsRepo "inventario/repositories/products"
	repo "inventario/repositories/stock"
)

// newAdjustService arma el Service para los ajustes. Existen las sucursales 1, 2 y 3.
// El producto 1 tiene en la sucursal 3 un total de 10 con 4 reservadas; el producto 2 no tiene stock.
func newAdjustService() (StockService, repo.StockChangesMock) {
	changes := repo.StockChangesMock{
		Stock: map[string]stock.StockRow{
			repo.StockKey(1, 3): {ProductID: 1, BranchID: 3, Total: 10, Reserved: 4},
			repo.StockKey(1, 1): {ProductID: 1, BranchID: 1, Total: 2},
		},
		Reservations: map[string]reservations.ReservationModel{},
		Movements:    &[]movements.MovementModel{},
	}
	service := StockService{
		ProductsRepo: productsRepo.ProductsMock{MockDB: []products.ProductModel{{ID: 1, Name: "Harina 000"}, {ID: 2, Name: "Azúcar"}}},
		Changes:      changes,
		Branches:     branchesRepo.BranchesMock{Existing: map[uint]bool{1: true, 2: true, 3: true}},
	}
	return service, changes
}

func branch(id uint) *uint {
	return &id
}

func TestAdjust_EntryCreatesRowAndMovement(t *testing.T) {
	service, changes := newAdjustService()

	row, err := service.Adjust("DIRECTOR", nil, 2, 2, movements.TypeIn, 15, "Llegó mercadería", "")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if row.Total != 15 || changes.Stock[repo.StockKey(2, 2)].Total != 15 {
		t.Errorf("Se esperaba un total de 15, pero quedó: %+v", row)
	}
	if len(*changes.Movements) != 1 {
		t.Fatalf("Se esperaba un movimiento, pero hay %d", len(*changes.Movements))
	}
	movement := (*changes.Movements)[0]
	if movement.Reason != movements.ReasonAdjustment || movement.Description != "Llegó mercadería" || movement.ResultingTotal != 15 {
		t.Errorf("El movimiento no es el del ajuste: %+v", movement)
	}
}

// CA-16: una salida que dejaría el stock negativo se rechaza.
func TestAdjust_ExitBelowZero(t *testing.T) {
	service, changes := newAdjustService()

	_, err := service.Adjust("ADMIN_SUCURSAL", branch(1), 1, 1, movements.TypeOut, 3, "Rotura", "")

	if !errors.Is(err, ErrBelowReserved) {
		t.Errorf("Se esperaba ErrBelowReserved, pero se obtuvo: %v", err)
	}
	if changes.Stock[repo.StockKey(1, 1)].Total != 2 || len(*changes.Movements) != 0 {
		t.Errorf("El ajuste rechazado no tenía que cambiar nada")
	}
}

// CA-17: con 10 en total y 4 reservadas, una salida de 8 se rechaza porque solo hay 6 disponibles.
func TestAdjust_ExitOverReserved(t *testing.T) {
	service, _ := newAdjustService()

	_, err := service.Adjust("ADMIN_SUCURSAL", branch(3), 3, 1, movements.TypeOut, 8, "Rotura", "")

	if !errors.Is(err, ErrBelowReserved) {
		t.Errorf("Se esperaba ErrBelowReserved, pero se obtuvo: %v", err)
	}
}

func TestAdjust_ExitWithinAvailable(t *testing.T) {
	service, _ := newAdjustService()

	row, err := service.Adjust("ADMIN_SUCURSAL", branch(3), 3, 1, movements.TypeOut, 6, "Rotura", "")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if row.Total != 4 || row.Reserved != 4 {
		t.Errorf("Se esperaba 4 de total y 4 reservadas, pero quedó: %+v", row)
	}
}

// CA-21: el admin de la sucursal 3 no puede ajustar otra sucursal; el vendedor no puede ajustar nada.
func TestAdjust_NotAllowed(t *testing.T) {
	service, _ := newAdjustService()

	_, errAdmin := service.Adjust("ADMIN_SUCURSAL", branch(3), 1, 1, movements.TypeIn, 1, "Llegó mercadería", "")
	_, errSeller := service.Adjust("VENDEDOR", branch(1), 1, 1, movements.TypeIn, 1, "Llegó mercadería", "")

	if !errors.Is(errAdmin, ErrNotAllowed) || !errors.Is(errSeller, ErrNotAllowed) {
		t.Errorf("Se esperaba ErrNotAllowed en los dos casos, pero se obtuvo: %v y %v", errAdmin, errSeller)
	}
}

func TestAdjust_MissingReason(t *testing.T) {
	service, _ := newAdjustService()

	_, err := service.Adjust("DIRECTOR", nil, 1, 1, movements.TypeIn, 1, "   ", "")

	if !errors.Is(err, ErrInvalidAdjustment) {
		t.Errorf("Se esperaba ErrInvalidAdjustment, pero se obtuvo: %v", err)
	}
}

func TestAdjust_BranchNotFound(t *testing.T) {
	service, _ := newAdjustService()

	_, err := service.Adjust("DIRECTOR", nil, 9, 1, movements.TypeIn, 1, "Llegó mercadería", "")

	if !errors.Is(err, ErrBranchNotFound) {
		t.Errorf("Se esperaba ErrBranchNotFound, pero se obtuvo: %v", err)
	}
}

func TestAdjust_BranchesDown(t *testing.T) {
	service, changes := newAdjustService()
	service.Branches = branchesRepo.BranchesMock{Err: errors.New("timeout")}

	_, err := service.Adjust("DIRECTOR", nil, 1, 1, movements.TypeIn, 1, "Llegó mercadería", "")

	if !errors.Is(err, ErrBranchesUnavailable) {
		t.Errorf("Se esperaba ErrBranchesUnavailable, pero se obtuvo: %v", err)
	}
	if len(*changes.Movements) != 0 {
		t.Errorf("Sin poder validar la sucursal no tenía que hacerse el ajuste")
	}
}

func TestGetMovements_NewestFirst(t *testing.T) {
	service, _ := newAdjustService()
	service.Adjust("DIRECTOR", nil, 2, 1, movements.TypeIn, 10, "Primero", "")
	service.Adjust("DIRECTOR", nil, 2, 1, movements.TypeOut, 3, "Segundo", "")

	result, err := service.GetMovements(2, 1, 1, 20)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if result.Total != 2 || result.Movements[0].Description != "Segundo" || result.Movements[0].ResultingTotal != 7 {
		t.Errorf("Se esperaban 2 movimientos, el más nuevo primero: %+v", result.Movements)
	}
}
