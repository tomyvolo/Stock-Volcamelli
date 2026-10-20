package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gateway/controllers"
	"gateway/logs"
	"gateway/observability"
)

const stage = "gateway"

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", controllers.Health)

	// Todo lo que entra por /api recibe un TrackID nuevo y queda medido.
	api := router.Group("/api")
	api.Use(observability.RecordMetrics, observability.AddTrackID)

	// TODO(Tomy): completar el gateway.
	// 1. Log al recibir y al responder cada pedido, con logStore.Write(ctx.GetString("track_id"), stage, ...).
	// 2. Validar el JWT con golang-jwt/jwt/v5 (HS256, clave en JWT_SECRET), como en utils/jwt_middleware.go de DSW.
	//    401 UNAUTHORIZED si falta, no es válido o venció. El login (/api/auth/*) no pide token.
	// 3. Controlar el rol: INTEGRACION solo puede usar las 4 consultas del contrato; si no, 403 FORBIDDEN.
	// 4. Reenviar el pedido con X-Track-ID, X-User-ID, X-User-Role y X-User-Branch:
	//    /api/auth/*  -> USUARIOS_URL
	//    /api/products*, /api/branches/:id/stock, /api/branches/:id/products/* -> INVENTARIO_URL
	// 5. Si el servicio no responde en 2 segundos, 503 SERVICE_UNAVAILABLE.

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
