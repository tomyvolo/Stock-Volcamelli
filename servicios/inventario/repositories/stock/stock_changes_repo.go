package stock

import (
	"inventario/models/movements"
	"inventario/models/reservations"
	"inventario/models/stock"
)

// StockChangesRepo es lo que usan las operaciones que cambian el stock: reservar, confirmar, liberar y ajustar.
// Agrupa el stock, las reservas y los movimientos porque siempre cambian juntos, en la misma transacción.
type StockChangesRepo interface {
	// Transaction corre fn dentro de una transacción. Si fn devuelve un error, no se guarda nada.
	Transaction(fn func(tx StockChangesRepo) error) error

	// LockStock lee la fila con SELECT ... FOR UPDATE: nadie más la cambia hasta que termine la transacción.
	// Devuelve gorm.ErrRecordNotFound si el producto no tiene stock cargado en esa sucursal.
	LockStock(productID, branchID uint) (stock.StockRow, error)
	// SaveStock crea la fila si no existía o la actualiza.
	SaveStock(row stock.StockRow) error

	// LockReservation devuelve gorm.ErrRecordNotFound si la transferencia no tiene reserva.
	LockReservation(transferID string) (reservations.ReservationModel, error)
	SaveReservation(reservation reservations.ReservationModel) error

	// EventProcessed dice si ya procesamos el evento con ese id.
	EventProcessed(eventID string) (bool, error)
	// SaveProcessedEvent anota el evento como procesado. Si ya estaba anotado, no hace nada.
	SaveProcessedEvent(event reservations.ProcessedEventModel) error

	AddMovement(movement movements.MovementModel) error
	// GetMovements devuelve los movimientos de un producto en una sucursal, del más nuevo al más viejo.
	GetMovements(branchID, productID uint, page, size int) (movements.MovementListResult, error)
}
