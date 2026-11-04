package stock

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"inventario/models/movements"
	"inventario/models/stock"
	branchesRepo "inventario/repositories/branches"
	productsRepo "inventario/repositories/products"
	repo "inventario/repositories/stock"
)

var (
	ErrProductNotFound     = errors.New("producto no encontrado")
	ErrNotAllowed          = errors.New("no tiene permiso para ajustar el stock de esa sucursal")
	ErrInvalidAdjustment   = errors.New("el tipo tiene que ser ENTRADA o SALIDA, la cantidad mayor a 0 y el motivo no puede estar vacío ni pasar los 200 caracteres")
	ErrBranchNotFound      = errors.New("la sucursal no existe")
	ErrBranchesUnavailable = errors.New("sucursales no respondió")
	ErrBelowReserved       = errors.New("la salida dejaría el total por debajo de lo reservado")
)

// MaxDescriptionLength es el largo máximo del motivo de un ajuste, el de la columna description de stock_movements.
const MaxDescriptionLength = 200

type StockService struct {
	Repo repo.StockRepo
	// Lo usamos para saber si un producto sin stock en la sucursal existe o no.
	ProductsRepo productsRepo.ProductsRepo
	// Los ajustes cambian el stock y dejan un movimiento, en la misma transacción.
	Changes repo.StockChangesRepo
	// Antes de un ajuste, le preguntamos a Sucursales si la sucursal existe.
	Branches branchesRepo.BranchesRepo
}

func (s *StockService) GetBranchStock(branchID uint, lowStock bool) ([]stock.StockModel, error) {
	list, err := s.Repo.GetStockByBranch(branchID, lowStock)
	if err != nil {
		return nil, err
	}
	// Una sucursal sin productos se responde como lista vacía, no como null.
	if list == nil {
		list = []stock.StockModel{}
	}
	return list, nil
}

// GetProductStock devuelve el stock disponible de un producto en una sucursal.
// Si el producto existe pero no tiene stock cargado ahí, la cantidad es 0.
func (s *StockService) GetProductStock(branchID, productID uint) (stock.StockModel, error) {
	row, err := s.Repo.GetStockByProduct(branchID, productID)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return stock.StockModel{}, err
	}

	product, err := s.ProductsRepo.GetProductByID(productID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return stock.StockModel{}, ErrProductNotFound
	}
	if err != nil {
		return stock.StockModel{}, err
	}
	return stock.StockModel{BranchID: branchID, ProductID: productID, ProductName: product.Name, Quantity: 0}, nil
}

// Adjust registra una entrada o una salida manual, por ejemplo cuando llega mercadería o hay una rotura.
// La puede hacer el director en cualquier sucursal o el admin de sucursal en la suya. userRole y userBranch
// son los de quien lo pide, sacados de su token por el gateway.
func (s *StockService) Adjust(userRole string, userBranch *uint, branchID, productID uint, kind string, quantity int, description, trackID string) (stock.StockRow, error) {
	switch userRole {
	case "DIRECTOR":
	case "ADMIN_SUCURSAL":
		if userBranch == nil || *userBranch != branchID {
			return stock.StockRow{}, ErrNotAllowed
		}
	default:
		return stock.StockRow{}, ErrNotAllowed
	}

	if kind != movements.TypeIn && kind != movements.TypeOut {
		return stock.StockRow{}, ErrInvalidAdjustment
	}
	row, _, err := s.change(branchID, productID, kind, quantity, description, trackID)
	return row, err
}

// Increase y Decrease los usa el grupo consumidor en Inventario publicado, donde no hay gateway ni roles.
// Hacen lo mismo que un ajuste y devuelven el stock disponible que quedó, como la consulta de stock.
func (s *StockService) Increase(branchID, productID uint, quantity int, reason, trackID string) (stock.StockModel, error) {
	return s.changeAvailable(branchID, productID, movements.TypeIn, quantity, reason, trackID)
}

func (s *StockService) Decrease(branchID, productID uint, quantity int, reason, trackID string) (stock.StockModel, error) {
	return s.changeAvailable(branchID, productID, movements.TypeOut, quantity, reason, trackID)
}

func (s *StockService) changeAvailable(branchID, productID uint, kind string, quantity int, reason, trackID string) (stock.StockModel, error) {
	row, productName, err := s.change(branchID, productID, kind, quantity, reason, trackID)
	if err != nil {
		return stock.StockModel{}, err
	}
	return stock.StockModel{BranchID: row.BranchID, ProductID: row.ProductID, ProductName: productName, Quantity: row.Total - row.Reserved}, nil
}

// change valida el pedido y, en una transacción, cambia el stock y anota el movimiento.
// Devuelve también el nombre del producto, que Increase y Decrease ponen en la respuesta.
func (s *StockService) change(branchID, productID uint, kind string, quantity int, description, trackID string) (stock.StockRow, string, error) {
	description = strings.TrimSpace(description)
	if quantity <= 0 || description == "" || utf8.RuneCountInString(description) > MaxDescriptionLength {
		return stock.StockRow{}, "", ErrInvalidAdjustment
	}

	exists, err := s.Branches.Exists(branchID, trackID)
	if err != nil {
		return stock.StockRow{}, "", fmt.Errorf("%w: %v", ErrBranchesUnavailable, err)
	}
	if !exists {
		return stock.StockRow{}, "", ErrBranchNotFound
	}
	product, err := s.ProductsRepo.GetProductByID(productID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return stock.StockRow{}, "", ErrProductNotFound
	} else if err != nil {
		return stock.StockRow{}, "", err
	}

	var result stock.StockRow
	err = s.Changes.Transaction(func(tx repo.StockChangesRepo) error {
		row, err := tx.LockStock(productID, branchID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = stock.StockRow{ProductID: productID, BranchID: branchID}
		} else if err != nil {
			return err
		}

		if kind == movements.TypeIn {
			row.Total += quantity
		} else {
			// Las unidades reservadas ya están comprometidas en una transferencia: no se pueden sacar.
			if row.Total-quantity < row.Reserved {
				return ErrBelowReserved
			}
			row.Total -= quantity
		}

		if err := tx.SaveStock(row); err != nil {
			return err
		}
		result = row
		return tx.AddMovement(movements.MovementModel{
			ProductID: productID, BranchID: branchID, Type: kind, Quantity: quantity,
			ResultingTotal: row.Total, Reason: movements.ReasonAdjustment, Description: description,
		})
	})
	return result, product.Name, err
}

// GetMovements devuelve el historial de un producto en una sucursal, del más nuevo al más viejo.
func (s *StockService) GetMovements(branchID, productID uint, page, size int) (movements.MovementListResult, error) {
	if _, err := s.ProductsRepo.GetProductByID(productID); errors.Is(err, gorm.ErrRecordNotFound) {
		return movements.MovementListResult{}, ErrProductNotFound
	} else if err != nil {
		return movements.MovementListResult{}, err
	}
	result, err := s.Changes.GetMovements(branchID, productID, page, size)
	if err != nil {
		return movements.MovementListResult{}, err
	}
	// Sin movimientos se responde una lista vacía, no null.
	if result.Movements == nil {
		result.Movements = []movements.MovementModel{}
	}
	return result, nil
}
