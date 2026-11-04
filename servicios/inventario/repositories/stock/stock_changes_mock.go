package stock

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"inventario/models/movements"
	"inventario/models/reservations"
	"inventario/models/stock"
)

// StockChangesMock simula ser MySQL, pero vive solo en la RAM.
// La clave de Stock es "producto-sucursal" (ver StockKey) y la de Reservations, el id de la transferencia.
// Movements es un puntero para que los movimientos agregados se vean desde el test.
type StockChangesMock struct {
	Stock           map[string]stock.StockRow
	Reservations    map[string]reservations.ReservationModel
	Movements       *[]movements.MovementModel
	ProcessedEvents map[string]bool
}

func StockKey(productID, branchID uint) string {
	return fmt.Sprintf("%d-%d", productID, branchID)
}

// Transaction corre fn sobre el mismo mock. No deshace nada si fn falla: los services
// controlan todo antes de guardar, así que en los tests no hace falta.
func (repo StockChangesMock) Transaction(fn func(tx StockChangesRepo) error) error {
	return fn(repo)
}

func (repo StockChangesMock) LockStock(productID, branchID uint) (stock.StockRow, error) {
	row, exists := repo.Stock[StockKey(productID, branchID)]
	if !exists {
		return stock.StockRow{}, gorm.ErrRecordNotFound
	}
	return row, nil
}

func (repo StockChangesMock) SaveStock(row stock.StockRow) error {
	repo.Stock[StockKey(row.ProductID, row.BranchID)] = row
	return nil
}

func (repo StockChangesMock) LockReservation(transferID string) (reservations.ReservationModel, error) {
	reservation, exists := repo.Reservations[transferID]
	if !exists {
		return reservations.ReservationModel{}, gorm.ErrRecordNotFound
	}
	return reservation, nil
}

func (repo StockChangesMock) SaveReservation(reservation reservations.ReservationModel) error {
	repo.Reservations[reservation.TransferID] = reservation
	return nil
}

func (repo StockChangesMock) EventProcessed(eventID string) (bool, error) {
	return repo.ProcessedEvents[eventID], nil
}

func (repo StockChangesMock) SaveProcessedEvent(event reservations.ProcessedEventModel) error {
	repo.ProcessedEvents[event.EventID] = true
	return nil
}

func (repo StockChangesMock) AddMovement(movement movements.MovementModel) error {
	movement.ID = uint(len(*repo.Movements) + 1)
	movement.CreatedAt = time.Now()
	*repo.Movements = append(*repo.Movements, movement)
	return nil
}

func (repo StockChangesMock) GetMovements(branchID, productID uint, page, size int) (movements.MovementListResult, error) {
	var list []movements.MovementModel
	for i := len(*repo.Movements) - 1; i >= 0; i-- {
		movement := (*repo.Movements)[i]
		if movement.BranchID == branchID && movement.ProductID == productID {
			list = append(list, movement)
		}
	}
	total := int64(len(list))
	offset := (page - 1) * size
	if offset >= len(list) {
		return movements.MovementListResult{Total: total}, nil
	}
	end := min(offset+size, len(list))
	return movements.MovementListResult{Movements: list[offset:end], Total: total}, nil
}
