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
	ToleranceMinutes      int            `json:"toleranceMinutes"`
	Notes                 string         `json:"notes"`
	ProposedArrival       *time.Time     `json:"proposedArrival"`
	ProposalExpiresAt     *time.Time     `json:"proposalExpiresAt"`
	CommittedArrival      *time.Time     `json:"committedArrival"`
	DelayMinutes          int            `json:"delayMinutes"`
	Reason                string         `json:"reason"`
	// La cancelación o la respuesta al horario ya se pidieron y se le están mandando a Rumbo.
	CancellationRequested bool  `json:"cancellationRequested"`
	ScheduleResponse      *bool `json:"scheduleResponse"`
	// Rumbo no aprobó la última cancelación pendiente.
	CancellationRejected bool          `json:"cancellationRejected"`
	History              []historyJSON `json:"history"`
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
	return transferenciaJSON{
		ID: t.ID, ProductID: t.ProductoID, ProductName: t.NombreProducto, Quantity: t.Cantidad,
		OriginBranchID: t.OrigenID, OriginBranchName: t.Origen.Nombre,
		DestinationBranchID: t.DestinoID, DestinationBranchName: t.Destino.Nombre,
		CreatedBy: t.CreadaPor, CreatedAt: t.CreadaEn, Status: t.Estado,
		DesiredArrival: t.HoraLlegadaPedida, ToleranceMinutes: t.ToleranciaMinutos, Notes: t.Observaciones,
		ProposedArrival: t.HoraPropuesta, ProposalExpiresAt: t.VencePropuesta, CommittedArrival: t.HoraComprometida,
		DelayMinutes: t.DemoraMinutos, Reason: t.Motivo,
		CancellationRequested: t.CancelacionPorEnviar != nil, ScheduleResponse: t.RespuestaPorEnviar,
		CancellationRejected: t.CancelacionRechazada, History: history,
	}
}
