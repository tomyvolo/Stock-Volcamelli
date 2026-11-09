// Package events guarda los navegadores conectados y les reparte cada aviso que llega de RabbitMQ.
package events

import "sync"

// Event es un aviso listo para mandar al navegador: el tipo de evento y su cuerpo en JSON.
type Event struct {
	Type string
	Data []byte
}

// MaxPending es cuántos avisos sin enviar le aguantamos a un navegador. Si se le juntan más,
// lo desconectamos: él reconecta y recarga la pantalla, y así uno lento no frena a los demás.
const MaxPending = 20

// Hub tiene un canal por cada navegador conectado. Las conexiones viven en memoria,
// por eso Tiempo Real corre en una sola instancia.
type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]bool
}

func NewHub() *Hub {
	return &Hub{clients: map[chan Event]bool{}}
}

// Subscribe registra un navegador y devuelve el canal por el que le van a llegar los avisos.
func (h *Hub) Subscribe() chan Event {
	ch := make(chan Event, MaxPending)
	h.mu.Lock()
	h.clients[ch] = true
	h.mu.Unlock()
	return ch
}

// Unsubscribe saca al navegador cuando cierra la conexión. Si ya lo había sacado Broadcast, no hace nada.
func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.remove(ch)
}

// Broadcast le manda el aviso a todos los navegadores sin esperar a ninguno. Al que tiene el canal
// lleno lo saca y le cierra el canal, y así la conexión de ese navegador se corta.
func (h *Hub) Broadcast(event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			h.remove(ch)
		}
	}
}

// Count devuelve cuántos navegadores hay conectados. Lo muestra la métrica de conexiones abiertas.
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// remove se llama con el lock tomado.
func (h *Hub) remove(ch chan Event) {
	if h.clients[ch] {
		delete(h.clients, ch)
		close(ch)
	}
}
