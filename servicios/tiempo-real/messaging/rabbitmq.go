// Package messaging escucha en RabbitMQ los eventos que le interesan al frontend y se los pasa al Hub.
package messaging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"tiempo-real/events"
	"tiempo-real/logs"
)

const stage = "tiempo-real"

// Cuánto esperamos antes de volver a conectar con RabbitMQ.
const retryWait = 5 * time.Second

// Exchange es el exchange fanout de un tipo de evento, el mismo que declara quien publica.
func Exchange(eventType string) string {
	return "ex." + eventType
}

// Listen escucha un tipo de evento y le pasa cada aviso al Hub. Si se corta la conexión, vuelve a conectar.
func Listen(url, eventType string, hub *events.Hub, logStore *logs.Store) {
	for {
		err := consume(url, eventType, hub, logStore)
		log.Printf("dejamos de escuchar %s: %v. Volvemos a conectar en %v", eventType, err, retryWait)
		time.Sleep(retryWait)
	}
}

func consume(url, eventType string, hub *events.Hub, logStore *logs.Store) error {
	conn, err := amqp.Dial(url)
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
	// A diferencia de las colas de Inventario, esta no queda guardada: se borra cuando nos desconectamos.
	// Si Tiempo Real estuvo caído, los avisos de ese rato no le sirven a nadie, porque los navegadores
	// recargan al reconectar. Por lo mismo no tiene cola de errores ni confirmamos cada mensaje.
	queue := "q.tiempo-real." + eventType
	if _, err := ch.QueueDeclare(queue, false, true, true, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(queue, "", Exchange(eventType), false, nil); err != nil {
		return err
	}
	deliveries, err := ch.Consume(queue, "", true, true, false, false, nil)
	if err != nil {
		return err
	}

	log.Printf("escuchando %s en la cola %s", eventType, queue)
	for delivery := range deliveries {
		data, err := compact(delivery.Body)
		if err != nil {
			logStore.Write("", stage, "warn", "se descartó un "+eventType+" que no es JSON: "+string(delivery.Body))
			continue
		}
		hub.Broadcast(events.Event{Type: eventType, Data: data})
	}
	return errors.New("RabbitMQ cerró la conexión")
}

// compact deja el JSON en una sola línea, que es como lo tiene que recibir el navegador en SSE.
func compact(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, body); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
