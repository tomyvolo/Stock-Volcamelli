package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// fakeRealtime hace de Tiempo Real: manda un aviso, espera y manda otro, y deja la conexión abierta.
// Guarda los headers que le llegaron para revisar qué le pasó el gateway.
func fakeRealtime(received *http.Header) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*received = r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event: StockActualizado\ndata: {\"disponible\":1}\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, "event: StockActualizado\ndata: {\"disponible\":2}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
}

// gatewayWith arma el gateway con un usuario ya autenticado cuyo token vence en expiresIn.
func gatewayWith(target string, expiresIn time.Duration) *httptest.Server {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/events", func(ctx *gin.Context) {
		ctx.Set("track_id", "track-1")
		ctx.Set("user_id", uint(7))
		ctx.Set("role", "VENDEDOR")
		ctx.Set("token_expires", time.Now().Add(expiresIn))
	}, NewProxy(nil).Stream("tiempo-real", target))
	return httptest.NewServer(router)
}

// Los avisos llegan de a uno, sin esperar a que Tiempo Real cierre la respuesta.
func TestStream_ForwardsEachMessage(t *testing.T) {
	var received http.Header
	realtime := fakeRealtime(&received)
	defer realtime.Close()
	gateway := gatewayWith(realtime.URL, time.Minute)
	defer gateway.Close()

	response, err := http.Get(gateway.URL + "/api/events")
	if err != nil {
		t.Fatalf("No se pudo conectar: %v", err)
	}
	defer response.Body.Close()

	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Se esperaba text/event-stream, pero vino %q", response.Header.Get("Content-Type"))
	}
	reader := bufio.NewReader(response.Body)
	for _, want := range []string{"event: StockActualizado\n", "data: {\"disponible\":1}\n", "\n", "event: StockActualizado\n", "data: {\"disponible\":2}\n"} {
		if line, _ := reader.ReadString('\n'); line != want {
			t.Fatalf("Se esperaba %q, pero llegó %q", want, line)
		}
	}
	if received.Get("X-Track-ID") != "track-1" || received.Get("X-User-ID") != "7" || received.Get("X-User-Role") != "VENDEDOR" {
		t.Errorf("A Tiempo Real no le llegaron el TrackID y el usuario: %v", received)
	}
}

// Cuando vence el token, el gateway corta la conexión.
func TestStream_ClosesWhenTokenExpires(t *testing.T) {
	var received http.Header
	realtime := fakeRealtime(&received)
	defer realtime.Close()
	gateway := gatewayWith(realtime.URL, 200*time.Millisecond)
	defer gateway.Close()

	response, err := http.Get(gateway.URL + "/api/events")
	if err != nil {
		t.Fatalf("No se pudo conectar: %v", err)
	}
	defer response.Body.Close()

	done := make(chan struct{})
	go func() {
		io.Copy(io.Discard, response.Body)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("La conexión seguía abierta después de vencer el token")
	}
}
