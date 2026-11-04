package messaging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"inventario/logs"
)

const EventProductUpdated = "ProductoActualizado"

// productEvent es lo que viaja en el cuerpo de ProductoActualizado.
type productEvent struct {
	ID        string `json:"id"`
	Type      string `json:"tipo"`
	ProductID uint   `json:"productoId"`
	TrackID   string `json:"trackId"`
	Date      string `json:"fecha"`
}

// RabbitMQPublisher publica los eventos de Inventario, cada uno en su exchange fanout.
type RabbitMQPublisher struct {
	URL  string
	Logs *logs.Store
}

// ProductUpdated publica en segundo plano, así el usuario no espera los reintentos si RabbitMQ no responde.
func (p RabbitMQPublisher) ProductUpdated(productID uint, trackID string) {
	event := productEvent{ID: newEventID(), Type: EventProductUpdated, ProductID: productID, TrackID: trackID, Date: time.Now().Format(time.RFC3339)}
	go p.publish(EventProductUpdated, trackID, event)
}

// publish reintenta después de 1, 2 y 4 segundos. Si después del cuarto intento sigue sin poder,
// deja el evento completo en el log de error para publicarlo a mano.
func (p RabbitMQPublisher) publish(eventType, trackID string, event any) {
	body, err := json.Marshal(event)
	if err != nil {
		p.Logs.Write(trackID, stage, "error", fmt.Sprintf("no se pudo armar %s: %v", eventType, err))
		return
	}
	for attempt := 0; ; attempt++ {
		err = p.send(eventType, body)
		if err == nil {
			return
		}
		if attempt == len(backoff) {
			break
		}
		time.Sleep(backoff[attempt])
	}
	p.Logs.Write(trackID, stage, "error", fmt.Sprintf("no se pudo publicar %s después de %d intentos: %v. Evento: %s", eventType, len(backoff)+1, err, body))
}

// send abre una conexión por evento, como el productor de la clase. Los productos cambian poco,
// así que no hace falta mantenerla abierta.
func (p RabbitMQPublisher) send(eventType string, body []byte) error {
	conn, err := amqp.Dial(p.URL)
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(Exchange(eventType), "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return ch.PublishWithContext(ctx, Exchange(eventType), "", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func newEventID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NullPublisher se usa cuando no hay RabbitMQ: el producto se guarda igual y el aviso queda en el log.
type NullPublisher struct{}

func (NullPublisher) ProductUpdated(productID uint, trackID string) {
	log.Printf("sin RabbitMQ: no se publicó %s del producto %d", EventProductUpdated, productID)
}
