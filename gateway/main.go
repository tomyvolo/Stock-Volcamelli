package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gateway/controllers"
	"gateway/logs"
	"gateway/observability"
	"gateway/proxy"
	"gateway/utils"
)

const stage = "gateway"

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}

	usersURL := os.Getenv("USUARIOS_URL")
	if usersURL == "" {
		usersURL = "http://localhost:8081"
	}
	// En Docker apunta al NGINX que reparte entre las dos instancias de Inventario.
	inventoryURL := os.Getenv("INVENTARIO_URL")
	if inventoryURL == "" {
		inventoryURL = "http://localhost:8082"
	}
	gatewayProxy := proxy.NewProxy(logStore)

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", controllers.Health)

	// Todo lo que entra por /api recibe un TrackID nuevo, queda medido y deja un log al entrar y al salir.
	api := router.Group("/api")
	api.Use(observability.RecordMetrics, observability.AddTrackID, observability.LogRequest(logStore))

	// El login no pide token.
	api.POST("/auth/login", gatewayProxy.Forward("usuarios", usersURL))

	// Las consultas del contrato: las puede usar cualquier rol, también INTEGRACION.
	// Las rutas que no son del contrato van en otro grupo, sin INTEGRACION en RoleRequired.
	contract := api.Group("").Use(utils.AuthRequired(), utils.RoleRequired("DIRECTOR", "ADMIN_SUCURSAL", "VENDEDOR", "INTEGRACION"))
	contract.GET("/products", gatewayProxy.Forward("inventario", inventoryURL))
	contract.GET("/products/:productId", gatewayProxy.Forward("inventario", inventoryURL))
	contract.GET("/branches/:branchId/stock", gatewayProxy.Forward("inventario", inventoryURL))
	contract.GET("/branches/:branchId/products/:productId/stock", gatewayProxy.Forward("inventario", inventoryURL))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	logStore.Write("", stage, "info", "gateway iniciado en el puerto "+port)
	log.Printf("gateway listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
