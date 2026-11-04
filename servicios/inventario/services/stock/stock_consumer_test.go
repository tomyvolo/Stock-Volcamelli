package stock

import (
	"errors"
	"strings"
	"testing"

	branchesRepo "inventario/repositories/branches"
	repo "inventario/repositories/stock"
)

// Los aumentos y reducciones del grupo consumidor, con la lista fija de sucursales de Inventario publicado.
func newConsumerService() (StockService, repo.StockChangesMock) {
	service, changes := newAdjustService()
	service.Branches = branchesRepo.BranchesList{IDs: []uint{1, 2, 3}}
	return service, changes
}

func TestIncrease_CreatesRow(t *testing.T) {
	service, changes := newConsumerService()

	result, err := service.Increase(2, 2, 5, "Llegó mercadería", "")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if result.Quantity != 5 || result.ProductName != "Azúcar" || result.BranchID != 2 || result.ProductID != 2 {
		t.Errorf("Se esperaba 5 disponibles de Azúcar en la sucursal 2, pero vino: %+v", result)
	}
	if changes.Stock[repo.StockKey(2, 2)].Total != 5 || len(*changes.Movements) != 1 {
		t.Errorf("El aumento tenía que guardar el stock y un movimiento")
	}
}

// Con 10 en total y 4 reservadas, reducir 2 deja 4 disponibles.
func TestDecrease_ReturnsAvailable(t *testing.T) {
	service, _ := newConsumerService()

	result, err := service.Decrease(3, 1, 2, "Venta", "")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if result.Quantity != 4 {
		t.Errorf("Se esperaban 4 disponibles, pero vino: %+v", result)
	}
}

// CA-25: reducir más que el disponible se rechaza y no cambia nada.
func TestDecrease_MoreThanAvailable(t *testing.T) {
	service, changes := newConsumerService()

	_, err := service.Decrease(3, 1, 7, "Venta", "")

	if !errors.Is(err, ErrBelowReserved) {
		t.Errorf("Se esperaba ErrBelowReserved, pero se obtuvo: %v", err)
	}
	if changes.Stock[repo.StockKey(1, 3)].Total != 10 || len(*changes.Movements) != 0 {
		t.Errorf("La reducción rechazada no tenía que cambiar nada")
	}
}

func TestDecrease_ProductWithoutStock(t *testing.T) {
	service, _ := newConsumerService()

	_, err := service.Decrease(1, 2, 1, "Venta", "")

	if !errors.Is(err, ErrBelowReserved) {
		t.Errorf("Se esperaba ErrBelowReserved, pero se obtuvo: %v", err)
	}
}

func TestIncrease_BranchNotInList(t *testing.T) {
	service, _ := newConsumerService()

	_, err := service.Increase(7, 1, 1, "Llegó mercadería", "")

	if !errors.Is(err, ErrBranchNotFound) {
		t.Errorf("Se esperaba ErrBranchNotFound, pero se obtuvo: %v", err)
	}
}

func TestIncrease_ProductNotFound(t *testing.T) {
	service, _ := newConsumerService()

	_, err := service.Increase(1, 99, 1, "Llegó mercadería", "")

	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("Se esperaba ErrProductNotFound, pero se obtuvo: %v", err)
	}
}

func TestIncrease_InvalidData(t *testing.T) {
	service, _ := newConsumerService()

	_, errQuantity := service.Increase(1, 1, 0, "Llegó mercadería", "")
	_, errReason := service.Increase(1, 1, 1, strings.Repeat("a", MaxDescriptionLength+1), "")

	if !errors.Is(errQuantity, ErrInvalidAdjustment) || !errors.Is(errReason, ErrInvalidAdjustment) {
		t.Errorf("Se esperaba ErrInvalidAdjustment en los dos casos, pero se obtuvo: %v y %v", errQuantity, errReason)
	}
}
