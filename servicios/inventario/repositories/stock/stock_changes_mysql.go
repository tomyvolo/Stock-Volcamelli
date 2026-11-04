package stock

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"inventario/models/movements"
	"inventario/models/reservations"
	"inventario/models/stock"
)

type StockChangesMySQL struct {
	DB *gorm.DB
}

// Transaction le pasa a fn un repositorio que usa la transacción en vez de la conexión normal.
func (repo StockChangesMySQL) Transaction(fn func(tx StockChangesRepo) error) error {
	return repo.DB.Transaction(func(tx *gorm.DB) error {
		return fn(StockChangesMySQL{DB: tx})
	})
}

// LockStock bloquea la fila en MySQL y no en memoria, porque hay dos instancias de Inventario.
func (repo StockChangesMySQL) LockStock(productID, branchID uint) (stock.StockRow, error) {
	var row stock.StockRow
	err := repo.DB.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("product_id = ? AND branch_id = ?", productID, branchID).
		Take(&row).Error
	return row, err
}

// SaveStock usa INSERT ... ON DUPLICATE KEY UPDATE, así sirve para filas nuevas y para las que ya existen.
func (repo StockChangesMySQL) SaveStock(row stock.StockRow) error {
	return repo.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error
}

func (repo StockChangesMySQL) LockReservation(transferID string) (reservations.ReservationModel, error) {
	var reservation reservations.ReservationModel
	err := repo.DB.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("transfer_id = ?", transferID).
		Take(&reservation).Error
	return reservation, err
}

func (repo StockChangesMySQL) SaveReservation(reservation reservations.ReservationModel) error {
	return repo.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&reservation).Error
}

func (repo StockChangesMySQL) EventProcessed(eventID string) (bool, error) {
	var count int64
	err := repo.DB.Model(&reservations.ProcessedEventModel{}).Where("event_id = ?", eventID).Count(&count).Error
	return count > 0, err
}

// SaveProcessedEvent usa INSERT IGNORE: si dos instancias procesan el mismo evento a la vez, la segunda
// no falla. La reserva ya la protege el bloqueo de su fila, así que el stock cambia una sola vez.
func (repo StockChangesMySQL) SaveProcessedEvent(event reservations.ProcessedEventModel) error {
	return repo.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error
}

// AddMovement inserta el movimiento; MySQL completa el id y la fecha.
func (repo StockChangesMySQL) AddMovement(movement movements.MovementModel) error {
	return repo.DB.Omit("ID", "CreatedAt").Create(&movement).Error
}

func (repo StockChangesMySQL) GetMovements(branchID, productID uint, page, size int) (movements.MovementListResult, error) {
	var list []movements.MovementModel
	var total int64

	// Una consulta nueva para el COUNT y otra para la página: GORM no deja reusar la misma.
	query := func() *gorm.DB {
		return repo.DB.Model(&movements.MovementModel{}).Where("branch_id = ? AND product_id = ?", branchID, productID)
	}
	if err := query().Count(&total).Error; err != nil {
		return movements.MovementListResult{}, err
	}
	offset := (page - 1) * size
	if err := query().Order("id DESC").Limit(size).Offset(offset).Find(&list).Error; err != nil {
		return movements.MovementListResult{}, err
	}
	return movements.MovementListResult{Movements: list, Total: total}, nil
}
