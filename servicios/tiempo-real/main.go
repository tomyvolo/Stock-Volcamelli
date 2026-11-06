package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"tiempo-real/events"
	"tiempo-real/logs"
	"tiempo-real/messaging"
)

const stage = "tiempo-real"

// Los dos eventos que le interesan al frontend (ADR-019).
var eventTypes = []string{"StockActualizado", "TransferenciaActualizada"}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}
	// Sin RabbitMQ no llega ningún aviso, así que no tiene sentido arrancar.
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		log.Fatal("falta la variable de entorno RABBITMQ_URL")
	}

	hub := events.NewHub()
	for _, eventType := range eventTypes {
		go messaging.Listen(rabbitURL, eventType, hub, logStore)
	}

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "open_connections",
		Help: "Navegadores conectados recibiendo avisos.",
	}, func() float64 { return float64(hub.Count()) })

	router.Use(gin.Recovery())
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/api/events", logConnection(logStore), events.Stream(hub, events.KeepAlive))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}
	logStore.Write("", stage, "info", "tiempo real iniciado en el puerto "+port)
	log.Printf("tiempo real listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// logConnection deja un log cuando un navegador se conecta y otro cuando se va, con el TrackID del gateway.
func logConnection(logStore *logs.Store) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		trackID := ctx.GetHeader("X-Track-ID")
		start := time.Now()
		// En segundo plano: el gateway espera solo 2 segundos la respuesta, y un MySQL de logs lento no
		// tiene que dejar a la pantalla sin avisos.
		go logStore.Write(trackID, stage, "info", "se conectó el usuario "+ctx.GetHeader("X-User-ID"))
		ctx.Next()
		logStore.Write(trackID, stage, "info", fmt.Sprintf("se desconectó el usuario %s después de %s", ctx.GetHeader("X-User-ID"), time.Since(start).Round(time.Second)))
	}
}
