package messaging

import (
	"encoding/json"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"inventario/logs"
	"inventario/models/reservations"
	services "inventario/services/reservations"
)

var ErrUnreadableEvent = errors.New("el evento no se puede leer o le faltan datos")

// transferEvent es lo que publica Transferencias. Usamos solo los campos que necesitamos.
type transferEvent struct {
	ID         string `json:"id"`
	TransferID string `json:"transferenciaId"`
	TrackID    string `json:"trackId"`
}

// TransfersConsumer escucha TrasladoCompletado y TrasladoCancelado, y confirma o libera la reserva.
// Las dos instancias de Inventario consumen de la misma cola: RabbitMQ le da cada mensaje a una sola.
type TransfersConsumer struct {
	URL     string
	Service services.ReservationsService
	Logs    *logs.Store
}

// Start deja escuchando una cola por cada evento, cada una en su goroutine.
func (c *TransfersConsumer) Start() {
	for _, eventType := range []string{reservations.EventTransferCompleted, reservations.EventTransferCancelled} {
		go listen(c.URL, eventType, func(delivery amqp.Delivery) { c.handle(eventType, delivery) })
	}
}

// handle confirma el mensaje con ACK si se procesó. Si no se puede procesar nunca, lo rechaza y
// RabbitMQ lo pasa a la cola de errores. Si falló por algo pasajero, lo devuelve a la cola.
func (c *TransfersConsumer) handle(eventType string, delivery amqp.Delivery) {
	trackID, err := c.Process(eventType, delivery.Body)
	switch {
	case err == nil:
		c.Logs.Write(trackID, stage, "info", fmt.Sprintf("procesamos %s: %s", eventType, delivery.Body))
		ack(delivery)
	case Discard(err):
		c.Logs.Write(trackID, stage, "error", fmt.Sprintf("%s va a la cola de errores: %v. Evento: %s", eventType, err, delivery.Body))
		reject(delivery)
	default:
		c.Logs.Write(trackID, stage, "error", fmt.Sprintf("no se pudo procesar %s, lo intentamos de nuevo: %v", eventType, err))
		requeue(delivery)
	}
}

// Process lee el evento y confirma o libera la reserva. Devuelve el TrackID para los logs.
func (c *TransfersConsumer) Process(eventType string, body []byte) (string, error) {
	var event transferEvent
	if err := json.Unmarshal(body, &event); err != nil || event.ID == "" || event.TransferID == "" {
		return event.TrackID, ErrUnreadableEvent
	}
	if eventType == reservations.EventTransferCompleted {
		return event.TrackID, c.Service.ConfirmByEvent(event.ID, event.TransferID)
	}
	return event.TrackID, c.Service.ReleaseByEvent(event.ID, event.TransferID)
}

// Discard dice si el error no se arregla reintentando: el evento no se puede leer, o la reserva
// no existe o ya terminó de otra forma. Esos eventos los revisamos a mano en la cola de errores.
func Discard(err error) bool {
	return errors.Is(err, ErrUnreadableEvent) ||
		errors.Is(err, services.ErrInvalidReservation) ||
		errors.Is(err, services.ErrReservationNotFound) ||
		errors.Is(err, services.ErrReservationReleased) ||
		errors.Is(err, services.ErrReservationConfirmed)
}
