// Package rabbitmq publica los eventos de las transferencias. Cada tipo de evento tiene su exchange
// fanout, y cada servicio que lo consume enlaza su propia cola, sin que quien publica se entere.
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"transferencias/puertos"
)

// Esperas entre intentos: si RabbitMQ no responde, reintentamos después de 1, 2 y 4 segundos.
var esperas = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// mensaje es lo que viaja en el cuerpo del evento.
type mensaje struct {
	ID              string `json:"id"`
	Tipo            string `json:"tipo"`
	TransferenciaID string `json:"transferenciaId"`
	Estado          string `json:"estado"`
	OrigenID        uint   `json:"origenId"`
	DestinoID       uint   `json:"destinoId"`
	Motivo          string `json:"motivo,omitempty"`
	TrackID         string `json:"trackId"`
	Fecha           string `json:"fecha"`
}

// Publicador implementa puertos.Eventos. Si la conexión se cae, la vuelve a abrir en el próximo intento.
type Publicador struct {
	URL string

	mu       sync.Mutex
	conexion *amqp.Connection
	canal    *amqp.Channel
}

// Exchange es el nombre del exchange de un tipo de evento, por ejemplo ex.TrasladoCompletado.
func Exchange(tipo string) string {
	return "ex." + tipo
}

func (p *Publicador) Publicar(evento puertos.Evento) error {
	cuerpo, err := json.Marshal(mensaje{
		ID: evento.ID, Tipo: evento.Tipo, TransferenciaID: evento.TransferenciaID, Estado: string(evento.Estado),
		OrigenID: evento.OrigenID, DestinoID: evento.DestinoID, Motivo: evento.Motivo, TrackID: evento.TrackID, Fecha: time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	for intento := 0; ; intento++ {
		err = p.intentar(evento.Tipo, evento.ID, cuerpo)
		if err == nil || intento == len(esperas) {
			break
		}
		time.Sleep(esperas[intento])
	}
	if err != nil {
		// Después del cuarto intento, el evento completo queda en el log de error para publicarlo a mano.
		return fmt.Errorf("después de %d intentos: %v. Evento: %s", len(esperas)+1, err, cuerpo)
	}
	return nil
}

func (p *Publicador) intentar(tipo, id string, cuerpo []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.canal == nil || p.canal.IsClosed() {
		if err := p.conectar(); err != nil {
			return err
		}
	}
	if err := p.canal.ExchangeDeclare(Exchange(tipo), "fanout", true, false, false, false, nil); err != nil {
		p.canal = nil
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := p.canal.PublishWithContext(ctx, Exchange(tipo), "", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    id,
		Body:         cuerpo,
	})
	if err != nil {
		p.canal = nil
	}
	return err
}

func (p *Publicador) conectar() error {
	if p.conexion == nil || p.conexion.IsClosed() {
		conexion, err := amqp.Dial(p.URL)
		if err != nil {
			return err
		}
		p.conexion = conexion
	}
	canal, err := p.conexion.Channel()
	if err != nil {
		p.conexion.Close()
		p.conexion = nil
		return err
	}
	p.canal = canal
	return nil
}
