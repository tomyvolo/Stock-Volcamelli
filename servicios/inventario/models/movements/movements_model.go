package movements

import "time"

// Tipos y motivos de un movimiento.
const (
	TypeIn  = "ENTRADA"
	TypeOut = "SALIDA"

	ReasonAdjustment = "AJUSTE"
	ReasonTransfer   = "TRANSFERENCIA"
)

// MovementModel es una fila de la tabla stock_movements: un cambio en el total de stock.
// Description es el motivo que escribió quien hizo el ajuste; TransferID, la transferencia que lo causó.
type MovementModel struct {
	ID             uint      `json:"id"`
	ProductID      uint      `json:"productId"`
	BranchID       uint      `json:"branchId"`
	Type           string    `json:"type"`
	Quantity       int       `json:"quantity"`
	ResultingTotal int       `json:"resultingTotal"`
	Reason         string    `json:"reason"`
	Description    string    `json:"description"`
	TransferID     *string   `json:"transferId"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (MovementModel) TableName() string {
	return "stock_movements"
}

// MovementListResult es una página de movimientos y la cantidad total.
type MovementListResult struct {
	Movements []MovementModel
	Total     int64
}
