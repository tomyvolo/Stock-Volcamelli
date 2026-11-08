package reservations

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"inventario/models/movements"
	"inventario/models/reservations"
	"inventario/models/stock"
	productsRepo "inventario/repositories/products"
	stockRepo "inventario/repositories/stock"
)

var (
	ErrInvalidReservation   = errors.New("faltan datos de la reserva, la cantidad no es mayor a 0 o el origen y el destino son la misma sucursal")
	ErrProductNotFound      = errors.New("producto no encontrado")
	ErrInsufficientStock    = errors.New("no hay stock disponible suficiente en el origen")
	ErrReservationNotFound  = errors.New("no hay una reserva para esa transferencia")
	ErrReservationReleased  = errors.New("la reserva de esa transferencia ya fue liberada")
	ErrReservationConfirmed = errors.New("la reserva de esa transferencia ya fue confirmada")
)

// ReservationsService tiene las tres operaciones que usa Transferencias: reservar, confirmar y liberar.
// Cada una es una transacción de MySQL, y todas son idempotentes por id de transferencia:
// si el mismo pedido llega dos veces, la segunda no cambia nada.
type ReservationsService struct {
	Repo         stockRepo.StockChangesRepo
	ProductsRepo productsRepo.ProductsRepo
	// Avisa cada cambio del disponible, para que Tiempo Real actualice las pantallas. Puede ser nil.
	Publisher Publisher
}

// Publisher publica StockActualizado. Lo implementa messaging con RabbitMQ.
type Publisher interface {
	StockUpdated(branchID, productID uint, available int, trackID string)
}

// notify avisa el disponible de cada fila que cambió. Se llama recién cuando la transacción se guardó.
func (s *ReservationsService) notify(rows []stock.StockRow) {
	if s.Publisher == nil {
		return
	}
	for _, row := range rows {
		s.Publisher.StockUpdated(row.BranchID, row.ProductID, row.Total-row.Reserved, "")
	}
}

// Reserve aparta unidades en el origen. Si la transferencia ya tenía su reserva, la devuelve sin reservar de nuevo.
// Si ya se había liberado (porque el pedido de liberar llegó antes), rechaza la reserva.
func (s *ReservationsService) Reserve(request reservations.ReservationModel) (reservations.ReservationModel, error) {
	if request.TransferID == "" || request.ProductID == 0 || request.OriginBranchID == 0 || request.DestinationBranchID == 0 ||
		request.OriginBranchID == request.DestinationBranchID || request.Quantity <= 0 {
		return reservations.ReservationModel{}, ErrInvalidReservation
	}

	var result reservations.ReservationModel
	var changed []stock.StockRow
	err := s.Repo.Transaction(func(tx stockRepo.StockChangesRepo) error {
		existing, err := tx.LockReservation(request.TransferID)
		if err == nil {
			if existing.Status == reservations.StatusReleased {
				return ErrReservationReleased
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		row, err := tx.LockStock(request.ProductID, request.OriginBranchID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Sin fila de stock: o el producto no existe, o no tiene unidades en el origen.
			if _, err := s.ProductsRepo.GetProductByID(request.ProductID); errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductNotFound
			} else if err != nil {
				return err
			}
			return ErrInsufficientStock
		}
		if err != nil {
			return err
		}
		if row.Total-row.Reserved < request.Quantity {
			return ErrInsufficientStock
		}

		row.Reserved += request.Quantity
		if err := tx.SaveStock(row); err != nil {
			return err
		}
		request.Status = reservations.StatusActive
		if err := tx.SaveReservation(request); err != nil {
			return err
		}
		result = request
		changed = []stock.StockRow{row}
		return nil
	})
	if err == nil {
		s.notify(changed)
	}
	return result, err
}

// Release devuelve las unidades reservadas al disponible del origen.
// Si la transferencia no tenía reserva, la anota como liberada, así un pedido de reserva que llega tarde se rechaza.
func (s *ReservationsService) Release(transferID string) (reservations.ReservationModel, error) {
	if transferID == "" {
		return reservations.ReservationModel{}, ErrInvalidReservation
	}

	var result reservations.ReservationModel
	var changed []stock.StockRow
	err := s.Repo.Transaction(func(tx stockRepo.StockChangesRepo) error {
		var err error
		result, changed, err = release(tx, transferID)
		return err
	})
	if err == nil {
		s.notify(changed)
	}
	return result, err
}

// ConfirmByEvent confirma la reserva cuando llega TrasladoCompletado.
func (s *ReservationsService) ConfirmByEvent(eventID, transferID string) error {
	return s.oncePerEvent(eventID, reservations.EventTransferCompleted, transferID, confirm)
}

// ReleaseByEvent libera la reserva cuando llega TrasladoCancelado.
func (s *ReservationsService) ReleaseByEvent(eventID, transferID string) error {
	return s.oncePerEvent(eventID, reservations.EventTransferCancelled, transferID, func(tx stockRepo.StockChangesRepo, transferID string) ([]stock.StockRow, error) {
		_, changed, err := release(tx, transferID)
		return changed, err
	})
}

// oncePerEvent corre change y anota el evento como procesado, en la misma transacción.
// Si RabbitMQ entrega el mismo evento otra vez, ya está anotado y no se cambia nada.
func (s *ReservationsService) oncePerEvent(eventID, eventType, transferID string,
	change func(tx stockRepo.StockChangesRepo, transferID string) ([]stock.StockRow, error)) error {
	if eventID == "" || transferID == "" {
		return ErrInvalidReservation
	}
	var changed []stock.StockRow
	err := s.Repo.Transaction(func(tx stockRepo.StockChangesRepo) error {
		processed, err := tx.EventProcessed(eventID)
		if err != nil {
			return err
		}
		if processed {
			return nil
		}
		if changed, err = change(tx, transferID); err != nil {
			return err
		}
		return tx.SaveProcessedEvent(reservations.ProcessedEventModel{EventID: eventID, Type: eventType})
	})
	if err == nil {
		s.notify(changed)
	}
	return err
}

// release devuelve también la fila de stock que cambió, o ninguna si no había nada que liberar.
func release(tx stockRepo.StockChangesRepo, transferID string) (reservations.ReservationModel, []stock.StockRow, error) {
	existing, err := tx.LockReservation(transferID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		released := reservations.ReservationModel{TransferID: transferID, Status: reservations.StatusReleased}
		return released, nil, tx.SaveReservation(released)
	}
	if err != nil {
		return reservations.ReservationModel{}, nil, err
	}

	switch existing.Status {
	case reservations.StatusReleased:
		return existing, nil, nil
	case reservations.StatusConfirmed:
		return reservations.ReservationModel{}, nil, ErrReservationConfirmed
	}

	row, err := tx.LockStock(existing.ProductID, existing.OriginBranchID)
	if err != nil {
		return reservations.ReservationModel{}, nil, err
	}
	row.Reserved -= existing.Quantity
	if err := tx.SaveStock(row); err != nil {
		return reservations.ReservationModel{}, nil, err
	}
	existing.Status = reservations.StatusReleased
	if err := tx.SaveReservation(existing); err != nil {
		return reservations.ReservationModel{}, nil, err
	}
	return existing, []stock.StockRow{row}, nil
}

// confirm pasa las unidades al destino cuando el traslado llegó: resta del total y de lo reservado
// en el origen, suma al total del destino y registra un movimiento en cada sucursal.
// Devuelve las dos filas de stock que cambió.
func confirm(tx stockRepo.StockChangesRepo, transferID string) ([]stock.StockRow, error) {
	reservation, err := tx.LockReservation(transferID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrReservationNotFound
	}
	if err != nil {
		return nil, err
	}
	switch reservation.Status {
	case reservations.StatusConfirmed:
		return nil, nil
	case reservations.StatusReleased:
		return nil, ErrReservationReleased
	}

	// Bloqueamos las dos filas siempre en el mismo orden (primero la sucursal de id menor),
	// así dos confirmaciones entre las mismas sucursales en sentidos opuestos no se traban entre sí.
	branchIDs := []uint{reservation.OriginBranchID, reservation.DestinationBranchID}
	if branchIDs[0] > branchIDs[1] {
		branchIDs[0], branchIDs[1] = branchIDs[1], branchIDs[0]
	}
	rows := map[uint]stock.StockRow{}
	for _, branchID := range branchIDs {
		row, err := tx.LockStock(reservation.ProductID, branchID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// El destino puede no tener el producto todavía: la fila se crea al guardar.
			row = stock.StockRow{ProductID: reservation.ProductID, BranchID: branchID}
		} else if err != nil {
			return nil, err
		}
		rows[branchID] = row
	}

	origin := rows[reservation.OriginBranchID]
	origin.Total -= reservation.Quantity
	origin.Reserved -= reservation.Quantity
	destination := rows[reservation.DestinationBranchID]
	destination.Total += reservation.Quantity

	if err := tx.SaveStock(origin); err != nil {
		return nil, err
	}
	if err := tx.SaveStock(destination); err != nil {
		return nil, err
	}

	out := movements.MovementModel{
		ProductID: reservation.ProductID, BranchID: origin.BranchID, Type: movements.TypeOut,
		Quantity: reservation.Quantity, ResultingTotal: origin.Total, Reason: movements.ReasonTransfer,
		Description: fmt.Sprintf("Hacia la sucursal %d", destination.BranchID), TransferID: &transferID,
	}
	in := movements.MovementModel{
		ProductID: reservation.ProductID, BranchID: destination.BranchID, Type: movements.TypeIn,
		Quantity: reservation.Quantity, ResultingTotal: destination.Total, Reason: movements.ReasonTransfer,
		Description: fmt.Sprintf("Desde la sucursal %d", origin.BranchID), TransferID: &transferID,
	}
	if err := tx.AddMovement(out); err != nil {
		return nil, err
	}
	if err := tx.AddMovement(in); err != nil {
		return nil, err
	}

	reservation.Status = reservations.StatusConfirmed
	return []stock.StockRow{origin, destination}, tx.SaveReservation(reservation)
}
