package logs

import (
	"bytes"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// hungStore arma un Store contra un servidor que acepta la conexión y nunca responde,
// como un MySQL de logs saturado.
func hungStore(t *testing.T) *Store {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("No se pudo abrir el puerto: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { conn.Close() })
		}
	}()

	dsn := "demo:demo@tcp(" + listener.Addr().String() + ")/logs?parseTime=true"
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: dsn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("No se pudo armar GORM: %v", err)
	}
	return &Store{db: db}
}

// Si MySQL no responde, el pedido no espera más que writeTimeout y el log queda completo en la consola.
func TestWrite_DoesNotBlockWhenMySQLHangs(t *testing.T) {
	store := hungStore(t)
	var console bytes.Buffer
	log.SetOutput(&console)
	defer log.SetOutput(os.Stderr)

	start := time.Now()
	store.Write("track-1", "gateway", "error", "mensaje que no se tiene que perder")
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("Se esperaba que Write volviera enseguida, pero tardó %v", elapsed)
	}
	for _, want := range []string{"track-1", "gateway", "error", "mensaje que no se tiene que perder"} {
		if !strings.Contains(console.String(), want) {
			t.Errorf("En la consola tenía que estar %q, pero quedó: %s", want, console.String())
		}
	}
}
