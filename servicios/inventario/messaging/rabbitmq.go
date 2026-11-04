package messaging

import (
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const stage = "inventario"

// Cuánto esperamos antes de volver a conectar con RabbitMQ, o antes de devolver a la cola un evento
// que falló por algo pasajero (por ejemplo, MySQL no respondió).
const retryWait = 5 * time.Second

// Esperas entre intentos cuando publicamos o cuando Solr no responde: 1, 2 y 4 segundos.
var backoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// Exchange es el exchange fanout de un tipo de evento, por ejemplo ex.TrasladoCompletado.
func Exchange(eventType string) string {
	return "ex." + eventType
}

// listen escucha la cola de Inventario para un tipo de evento y le pasa cada mensaje a handle.
// Si se corta la conexión con RabbitMQ, vuelve a conectar.
func listen(url, eventType string, handle func(amqp.Delivery)) {
	for {
		err := consume(url, eventType, handle)
		log.Printf("dejamos de escuchar %s: %v. Volvemos a conectar en %v", eventType, err, retryWait)
		time.Sleep(retryWait)
	}
}

func consume(url, eventType string, handle func(amqp.Delivery)) error {
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

	queue := "q.inventario." + eventType
	dlq := queue + ".dlq"

	// El mismo exchange fanout que declara quien publica: si arranca antes Inventario, ya existe.
	if err := ch.ExchangeDeclare(Exchange(eventType), "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	// Primero la cola de errores, y después la cola principal, que le manda ahí lo que rechazamos.
	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return err
	}
	args := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": dlq,
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, args); err != nil {
		return err
	}
	if err := ch.QueueBind(queue, "", Exchange(eventType), false, nil); err != nil {
		return err
	}
	// De a un mensaje por vez: el siguiente llega recién cuando confirmamos el anterior.
	if err := ch.Qos(1, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	log.Printf("escuchando %s en la cola %s", eventType, queue)
	for delivery := range deliveries {
		handle(delivery)
	}
	return errors.New("RabbitMQ cerró la conexión")
}

func ack(delivery amqp.Delivery) {
	if err := delivery.Ack(false); err != nil {
		log.Printf("no se pudo enviar el ACK: %v", err)
	}
}

// reject rechaza el mensaje sin volver a encolarlo: RabbitMQ lo pasa a la cola de errores.
func reject(delivery amqp.Delivery) {
	if err := delivery.Nack(false, false); err != nil {
		log.Printf("no se pudo enviar el NACK: %v", err)
	}
}

// requeue espera y devuelve el mensaje a la cola, para intentar de nuevo.
func requeue(delivery amqp.Delivery) {
	time.Sleep(retryWait)
	if err := delivery.Nack(false, true); err != nil {
		log.Printf("no se pudo enviar el NACK: %v", err)
	}
}
