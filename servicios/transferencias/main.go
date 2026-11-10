package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	adaptadorHTTP "transferencias/adaptadores/http"
	"transferencias/adaptadores/inventario"
	"transferencias/adaptadores/mongo"
	"transferencias/adaptadores/procesos"
	"transferencias/adaptadores/rabbitmq"
	"transferencias/adaptadores/registro"
	"transferencias/adaptadores/rumbo"
	"transferencias/adaptadores/sucursales"
	"transferencias/aplicacion"
)

func entorno(nombre, porDefecto string) string {
	if valor := os.Getenv(nombre); valor != "" {
		return valor
	}
	return porDefecto
}

func segundos(nombre string, porDefecto int) time.Duration {
	valor, err := strconv.Atoi(os.Getenv(nombre))
	if err != nil || valor <= 0 {
		valor = porDefecto
	}
	return time.Duration(valor) * time.Second
}

// nuevoID genera ids de 32 caracteres hexadecimales, como el TrackID. Sirve como Idempotency-Key para Rumbo.
func nuevoID() string {
	var bytes [16]byte
	rand.Read(bytes[:])
	return hex.EncodeToString(bytes[:])
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logs, err := registro.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}
	repo, err := mongo.Conectar(entorno("MONGO_URI", "mongodb://localhost:27017"), entorno("MONGO_DB", "transferencias"))
	if err != nil {
		log.Fatalf("no se pudo conectar a MongoDB: %v", err)
	}
	// Las llamadas a nuestros otros servicios esperan como mucho 2 segundos; a Rumbo, 5 (ADR-010).
	cliente := &http.Client{Timeout: 2 * time.Second}

	// Los adaptadores se arman acá y se le pasan al centro, que solo conoce los puertos.
	servicio := &aplicacion.Servicio{
		Repo: repo,
		// Inventario se consulta siempre a través de NGINX, que reparte entre sus dos instancias.
		Inventario: inventario.Inventario{BaseURL: entorno("INVENTARIO_URL", "http://localhost:8082"), Client: cliente},
		Sucursales: sucursales.Sucursales{BaseURL: entorno("SUCURSALES_URL", "http://localhost:8083"), Client: cliente},
		Eventos:    &rabbitmq.Publicador{URL: entorno("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")},
		Registro:   logs,
		Ahora:      time.Now,
		NuevoID:    nuevoID,
	}

	// Sin RUMBO_URL no le mandamos nada a Rumbo y las transferencias quedan RESERVADA. Así corremos las
	// pruebas de carga, sin ocupar el cupo de franjas que Rumbo comparte con los otros grupos (ADR-013).
	if rumboURL := os.Getenv("RUMBO_URL"); rumboURL != "" {
		if os.Getenv("RUMBO_CLIENT_ID") == "" || os.Getenv("RUMBO_CLIENT_SECRET") == "" {
			log.Fatal("con RUMBO_URL hacen falta también RUMBO_CLIENT_ID y RUMBO_CLIENT_SECRET")
		}
		servicio.Transporte = &rumbo.Cliente{
			BaseURL: rumboURL, ClientID: os.Getenv("RUMBO_CLIENT_ID"), ClientSecret: os.Getenv("RUMBO_CLIENT_SECRET"),
			HTTP: &http.Client{Timeout: rumbo.Timeout}, Registro: logs,
		}
	} else {
		logs.Escribir("", "warn", "sin RUMBO_URL: las transferencias se crean pero no se reservan en Rumbo")
	}
	controllers := adaptadorHTTP.Controllers{Servicio: servicio, Registro: logs}

	// Lo pendiente se manda cada 10 segundos; los cambios de Rumbo se consultan cada 60, como pide su contrato.
	procesos.Arrancar(servicio, segundos("ENVIO_SEGUNDOS", 10), segundos("CONSULTA_SEGUNDOS", 60))

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := router.Group("/api")
	api.Use(adaptadorHTTP.RecordMetrics, adaptadorHTTP.ReadTrackID)
	api.POST("/transfers", controllers.Crear)
	api.GET("/transfers", controllers.Listar)
	api.GET("/transfers/:transferId", controllers.Obtener)
	api.POST("/transfers/:transferId/cancel", controllers.Cancelar)

	port := entorno("PORT", "8084")
	logs.Escribir("", "info", "transferencias iniciado en el puerto "+port)
	log.Printf("transferencias listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
