package events

import "testing"

func TestBroadcast_ReachesEveryone(t *testing.T) {
	hub := NewHub()
	first, second := hub.Subscribe(), hub.Subscribe()

	hub.Broadcast(Event{Type: "StockActualizado", Data: []byte(`{"disponible":6}`)})

	for _, ch := range []chan Event{first, second} {
		if event := <-ch; event.Type != "StockActualizado" || string(event.Data) != `{"disponible":6}` {
			t.Errorf("Se esperaba el aviso de stock, pero llegó: %+v", event)
		}
	}
}

// Un navegador que no lee queda afuera, y los demás siguen recibiendo.
func TestBroadcast_DisconnectsSlowClient(t *testing.T) {
	hub := NewHub()
	slow, fast := hub.Subscribe(), hub.Subscribe()

	for i := 0; i <= MaxPending; i++ {
		hub.Broadcast(Event{Type: "StockActualizado"})
		<-fast
	}

	if hub.Count() != 1 {
		t.Fatalf("Se esperaba un solo navegador conectado, pero hay %d", hub.Count())
	}
	// El canal del lento queda cerrado después de los avisos que ya tenía: así su conexión se corta.
	for range slow {
	}
	hub.Broadcast(Event{Type: "TransferenciaActualizada"})
	if event := <-fast; event.Type != "TransferenciaActualizada" {
		t.Errorf("El navegador que lee tenía que seguir recibiendo, pero llegó: %+v", event)
	}
}

// Desconectarse dos veces (una por Broadcast y otra al cerrar la conexión) no rompe nada.
func TestUnsubscribe_Twice(t *testing.T) {
	hub := NewHub()
	ch := hub.Subscribe()

	hub.Unsubscribe(ch)
	hub.Unsubscribe(ch)

	if hub.Count() != 0 {
		t.Errorf("No tenía que quedar nadie conectado, pero hay %d", hub.Count())
	}
}
