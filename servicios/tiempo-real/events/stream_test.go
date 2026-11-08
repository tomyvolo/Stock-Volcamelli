package events

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// openStream levanta /api/events y se conecta como lo haría el navegador.
func openStream(t *testing.T, hub *Hub, keepAlive time.Duration) (*bufio.Reader, func()) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/events", Stream(hub, keepAlive))
	server := httptest.NewServer(router)

	response, err := http.Get(server.URL + "/api/events")
	if err != nil {
		t.Fatalf("No se pudo conectar: %v", err)
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Se esperaba 200 con text/event-stream, pero vino %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	return bufio.NewReader(response.Body), func() {
		response.Body.Close()
		server.Close()
	}
}

// readMessage lee hasta la línea vacía que cierra cada mensaje de SSE.
func readMessage(t *testing.T, reader *bufio.Reader) string {
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Se cortó la conexión: %v", err)
		}
		if line == "\n" {
			return strings.Join(lines, "")
		}
		lines = append(lines, line)
	}
}

func waitConnected(t *testing.T, hub *Hub, count int) {
	for i := 0; i < 100 && hub.Count() != count; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.Count() != count {
		t.Fatalf("Se esperaban %d navegadores conectados, pero hay %d", count, hub.Count())
	}
}

func TestStream_SendsEvent(t *testing.T) {
	hub := NewHub()
	reader, closeStream := openStream(t, hub, time.Minute)
	defer closeStream()
	waitConnected(t, hub, 1)

	hub.Broadcast(Event{Type: "StockActualizado", Data: []byte(`{"sucursalId":1,"productoId":3,"disponible":26}`)})

	want := "event: StockActualizado\ndata: {\"sucursalId\":1,\"productoId\":3,\"disponible\":26}\n"
	if got := readMessage(t, reader); got != want {
		t.Errorf("Se esperaba:\n%q\npero llegó:\n%q", want, got)
	}
}

func TestStream_KeepAlive(t *testing.T) {
	hub := NewHub()
	reader, closeStream := openStream(t, hub, 20*time.Millisecond)
	defer closeStream()

	if got := readMessage(t, reader); got != ":\n" {
		t.Errorf("Se esperaba el comentario vacío, pero llegó: %q", got)
	}
}

// Cuando el navegador cierra la conexión, sale de la lista.
func TestStream_ClientLeaves(t *testing.T) {
	hub := NewHub()
	_, closeStream := openStream(t, hub, time.Minute)
	waitConnected(t, hub, 1)

	closeStream()

	waitConnected(t, hub, 0)
}
