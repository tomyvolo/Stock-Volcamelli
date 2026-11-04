package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"inventario/logs"
	services "inventario/services/products"
)

// ProductsIndexer mantiene actualizada la búsqueda: escucha ProductoActualizado y reescribe el producto en Solr.
// Corre en una sola instancia de Inventario, así dos eventos del mismo producto no se procesan a la vez.
type ProductsIndexer struct {
	URL     string
	Service services.ProductsService
	Logs    *logs.Store
}

// Start primero reconstruye el índice desde MySQL (crea los campos si Solr arrancó vacío) y recién
// después empieza a escuchar los eventos. Si Solr todavía no responde, reintenta.
func (i *ProductsIndexer) Start() {
	go func() {
		for {
			written, err := i.Service.RebuildIndex()
			if err == nil {
				i.Logs.Write("", stage, "info", fmt.Sprintf("índice de búsqueda reconstruido con %d productos", written))
				break
			}
			log.Printf("no se pudo reconstruir el índice de búsqueda: %v. Reintentamos en %v", err, retryWait)
			time.Sleep(retryWait)
		}
		listen(i.URL, EventProductUpdated, i.handle)
	}()
}

// handle reintenta después de 1, 2 y 4 segundos. Si después del cuarto intento sigue fallando
// (por ejemplo, Solr está caído), el evento va a la cola de errores.
func (i *ProductsIndexer) handle(delivery amqp.Delivery) {
	var trackID string
	var err error
	for attempt := 0; ; attempt++ {
		trackID, err = i.Process(delivery.Body)
		if err == nil || i.discard(err) || attempt == len(backoff) {
			break
		}
		time.Sleep(backoff[attempt])
	}
	if err != nil {
		i.Logs.Write(trackID, stage, "error", fmt.Sprintf("%s va a la cola de errores: %v. Evento: %s", EventProductUpdated, err, delivery.Body))
		reject(delivery)
		return
	}
	i.Logs.Write(trackID, stage, "info", fmt.Sprintf("búsqueda actualizada: %s", delivery.Body))
	ack(delivery)
}

// Process lee el evento y reescribe el producto en Solr. Devuelve el TrackID para los logs.
func (i *ProductsIndexer) Process(body []byte) (string, error) {
	var event productEvent
	if err := json.Unmarshal(body, &event); err != nil || event.ProductID == 0 {
		return event.TrackID, ErrUnreadableEvent
	}
	return event.TrackID, i.Service.IndexProduct(event.ProductID)
}

// discard dice si reintentar no tiene sentido: el evento no se puede leer o el producto no existe.
func (i *ProductsIndexer) discard(err error) bool {
	return errors.Is(err, ErrUnreadableEvent) || errors.Is(err, services.ErrProductNotFound)
}
