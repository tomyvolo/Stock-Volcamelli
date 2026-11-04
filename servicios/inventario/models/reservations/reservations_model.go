package reservations

// Estados de una reserva.
const (
	StatusActive    = "ACTIVA"     // la transferencia todavía no terminó
	StatusConfirmed = "CONFIRMADA" // las unidades pasaron al destino
	StatusReleased  = "LIBERADA"   // las unidades volvieron a estar disponibles en el origen
)

// ReservationModel es una fila de la tabla reservations. El id es el de la transferencia.
type ReservationModel struct {
	TransferID          string `gorm:"primaryKey" json:"transferId"`
	ProductID           uint   `json:"productId"`
	OriginBranchID      uint   `json:"originBranchId"`
	DestinationBranchID uint   `json:"destinationBranchId"`
	Quantity            int    `json:"quantity"`
	Status              string `json:"status"`
}

func (ReservationModel) TableName() string {
	return "reservations"
}

// Eventos de Transferencias que cambian una reserva.
const (
	EventTransferCompleted = "TrasladoCompletado" // se confirma la reserva
	EventTransferCancelled = "TrasladoCancelado"  // se libera la reserva
)

// ProcessedEventModel es una fila de processed_events: el id de cada evento de Transferencias que ya
// procesamos. Se guarda en la misma transacción que confirma o libera la reserva.
type ProcessedEventModel struct {
	EventID string `gorm:"primaryKey"`
	Type    string
}

func (ProcessedEventModel) TableName() string {
	return "processed_events"
}
