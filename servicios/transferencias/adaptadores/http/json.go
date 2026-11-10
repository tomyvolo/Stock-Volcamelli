package http

import (
	"time"

	"transferencias/dominio"
)

// transferenciaJSON es cómo ve el frontend una transferencia. Los campos de Rumbo vienen vacíos
// mientras Rumbo no los informe.
type transferenciaJSON struct {
	ID                    string         `json:"id"`
	ProductID             uint           `json:"productId"`
	ProductName           string         `json:"productName"`
	Quantity              int            `json:"quantity"`
	OriginBranchID        uint           `json:"originBranchId"`
	OriginBranchName      string         `json:"originBranchName"`
	DestinationBranchID   uint           `json:"destinationBranchId"`
	DestinationBranchName string         `json:"destinationBranchName"`
	CreatedBy             uint           `json:"createdBy"`
	CreatedAt             time.Time      `json:"createdAt"`
	Status                dominio.Estado `json:"status"`
	DesiredArrival        *time.Time     `json:"desiredArrival"`
	Notes                 string         `json:"notes"`
	// La franja en la que Rumbo se comprometió a que llegue. Vacía mientras no la reserva.
	SlotStart    *time.Time `json:"slotStart"`
	SlotEnd      *time.Time `json:"slotEnd"`
	DelayMinutes int        `json:"delayMinutes"`
	Reason       string     `json:"reason"`
	// La cancelación ya se pidió y se le está mandando a Rumbo.
	CancellationRequested bool          `json:"cancellationRequested"`
	History               []historyJSON `json:"history"`
}

type historyJSON struct {
	Status dominio.Estado `json:"status"`
	Date   time.Time      `json:"date"`
	Detail string         `json:"detail"`
}

func aJSON(t *dominio.Transferencia) transferenciaJSON {
	history := make([]historyJSON, 0, len(t.Historial))
	for _, cambio := range t.Historial {
		history = append(history, historyJSON{Status: cambio.Estado, Date: cambio.Fecha, Detail: cambio.Detalle})
	}
	var slotStart, slotEnd *time.Time
	if t.Franja != nil && t.EnviadaARumbo() {
		slotStart, slotEnd = &t.Franja.Inicio, &t.Franja.Fin
	}
	return transferenciaJSON{
		ID: t.ID, ProductID: t.ProductoID, ProductName: t.NombreProducto, Quantity: t.Cantidad,
		OriginBranchID: t.OrigenID, OriginBranchName: t.Origen.Nombre,
		DestinationBranchID: t.DestinoID, DestinationBranchName: t.Destino.Nombre,
		CreatedBy: t.CreadaPor, CreatedAt: t.CreadaEn, Status: t.Estado,
		DesiredArrival: t.HoraLlegadaPedida, Notes: t.Observaciones,
		SlotStart: slotStart, SlotEnd: slotEnd, DelayMinutes: t.DemoraMinutos, Reason: t.Motivo,
		CancellationRequested: t.CancelacionPorEnviar != nil, History: history,
	}
}
