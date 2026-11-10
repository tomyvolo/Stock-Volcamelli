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
	// Sin la clave no se puede validar ningún token, así que no tiene sentido arrancar.
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("falta la variable de entorno JWT_SECRET")
	}

	usersURL := os.Getenv("USUARIOS_URL")
	if usersURL == "" {
		usersURL = "http://localhost:8081"
	}
	branchesURL := os.Getenv("SUCURSALES_URL")
	if branchesURL == "" {
		branchesURL = "http://localhost:8083"
	}
	transfersURL := os.Getenv("TRANSFERENCIAS_URL")
	if transfersURL == "" {
		transfersURL = "http://localhost:8084"
	}
	// En Docker apunta al NGINX que reparte entre las dos instancias de Inventario.
	inventoryURL := os.Getenv("INVENTARIO_URL")
	if inventoryURL == "" {
		inventoryURL = "http://localhost:8082"
	}
	realtimeURL := os.Getenv("TIEMPO_REAL_URL")
	if realtimeURL == "" {
		realtimeURL = "http://localhost:8085"
	}
	assistantURL := os.Getenv("ASISTENTE_URL")
	if assistantURL == "" {
		assistantURL = "http://localhost:8087"
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

	// Las consultas de Inventario: las puede usar cualquier rol. El grupo consumidor no entra por acá,
	// consulta directo a Inventario publicado con su API key.
	contract := api.Group("").Use(utils.AuthRequired(), utils.RoleRequired("DIRECTOR", "ADMIN_SUCURSAL", "VENDEDOR"))
	contract.GET("/products", gatewayProxy.Forward("inventario", inventoryURL))
	contract.GET("/products/:productId", gatewayProxy.Forward("inventario", inventoryURL))
	contract.GET("/branches/:branchId/stock", gatewayProxy.Forward("inventario", inventoryURL))
	contract.GET("/branches/:branchId/products/:productId/stock", gatewayProxy.Forward("inventario", inventoryURL))

	// Sucursales y usuarios: todos los roles los consultan.
	// Las rutas /branches/:branchId/stock y /branches/:branchId/products/... van a Inventario (arriba).
	allRoles := api.Group("").Use(utils.AuthRequired(), utils.RoleRequired("DIRECTOR", "ADMIN_SUCURSAL", "VENDEDOR"))
	allRoles.GET("/branches", gatewayProxy.Forward("sucursales", branchesURL))
	allRoles.GET("/branches/:branchId", gatewayProxy.Forward("sucursales", branchesURL))
	// La lista de usuarios de la empresa también la ve cualquier rol, sin contraseñas.
	allRoles.GET("/users", gatewayProxy.Forward("usuarios", usersURL))
	allRoles.GET("/branches/:branchId/products/:productId/movements", gatewayProxy.Forward("inventario", inventoryURL))
	allRoles.GET("/products/search", gatewayProxy.Forward("inventario", inventoryURL))

	// Transferencias: las usan los tres roles. Que el vendedor y el admin operen solo las que salen
	// de su sucursal o llegan a ella lo controla Transferencias, porque necesita los datos de la transferencia.
	allRoles.GET("/transfers", gatewayProxy.Forward("transferencias", transfersURL))
	allRoles.GET("/transfers/:transferId", gatewayProxy.Forward("transferencias", transfersURL))
	allRoles.POST("/transfers", gatewayProxy.TransferChange("transferencias", transfersURL))
	allRoles.POST("/transfers/:transferId/cancel", gatewayProxy.TransferChange("transferencias", transfersURL))

	// Solo el director: alta y cambios de sucursales y productos, y cambiar el rol o la sucursal de un usuario o borrarlo.
	directorOnly := api.Group("").Use(utils.AuthRequired(), utils.RoleRequired("DIRECTOR"))
	directorOnly.POST("/branches", gatewayProxy.Forward("sucursales", branchesURL))
	directorOnly.PUT("/branches/:branchId", gatewayProxy.Forward("sucursales", branchesURL))
	directorOnly.PUT("/users/:userId", gatewayProxy.Forward("usuarios", usersURL))
	directorOnly.DELETE("/users/:userId", gatewayProxy.Forward("usuarios", usersURL))
	directorOnly.POST("/products", gatewayProxy.Forward("inventario", inventoryURL))
	directorOnly.PUT("/products/:productId", gatewayProxy.Forward("inventario", inventoryURL))

	// El director y el admin de sucursal: alta de usuarios y ajustes de stock. Qué usuario puede crear
	// cada uno lo decide Usuarios, y que el admin ajuste solo su sucursal lo controla Inventario.
	directorOrAdmin := api.Group("").Use(utils.AuthRequired(), utils.RoleRequired("DIRECTOR", "ADMIN_SUCURSAL"))
	directorOrAdmin.POST("/create-users", gatewayProxy.Forward("usuarios", usersURL))
	directorOrAdmin.POST("/branches/:branchId/products/:productId/adjustments", gatewayProxy.Forward("inventario", inventoryURL))

	// Tiempo Real: la conexión queda abierta hasta una hora, así que no entra en las métricas de latencia
	// (arruinaría el p95 de todo lo demás). Tiene TrackID, logs y token como cualquier otro pedido.
	realtime := router.Group("/api")
	realtime.Use(observability.AddTrackID, observability.LogRequest(logStore), utils.AuthRequired(), utils.RoleRequired("DIRECTOR", "ADMIN_SUCURSAL", "VENDEDOR"))
	realtime.GET("/events", gatewayProxy.Stream("tiempo-real", realtimeURL))

	// Asistente: una pregunta puede tardar varios segundos, así que tampoco entra en las métricas de latencia.
	// El Asistente cuenta sus propias respuestas. Es la única ruta que recibe el token del usuario (ADR-018).
	assistant := router.Group("/api/assistant")
	assistant.Use(observability.AddTrackID, observability.LogRequest(logStore), utils.AuthRequired(), utils.RoleRequired("DIRECTOR", "ADMIN_SUCURSAL", "VENDEDOR"))
	assistant.POST("/chat", gatewayProxy.Assistant("asistente", assistantURL))
	assistant.POST("/confirm", gatewayProxy.Assistant("asistente", assistantURL))

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
