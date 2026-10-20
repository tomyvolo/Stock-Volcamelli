package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"usuarios/controllers"
	"usuarios/logs"
	"usuarios/observability"
)

const stage = "usuarios"

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}

	// TODO(Juan): conectar GORM al MySQL de Usuarios con DB_DSN y armar
	// repositories -> services -> controllers, como en el práctico.

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", controllers.Health)

	api := router.Group("/api")
	api.Use(observability.RecordMetrics, observability.ReadTrackID)

	// TODO(Juan): POST /api/auth/login
	// - Busca el usuario por email y compara la contraseña con bcrypt (utils/hash.go de DSW).
	// - Devuelve { "token": "...", "expiresIn": 3600 } con un JWT HS256 firmado con JWT_SECRET
	//   que lleve sub (id), rol, sucursal y exp (1 hora). Como utils/jwt.go de DSW.
	// - Si no coincide: 401 { "code": "INVALID_CREDENTIALS", "message": "..." }.
	// - Log con logStore.Write(ctx.GetString("track_id"), stage, ...), sin escribir nunca la contraseña.

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	logStore.Write("", stage, "info", "usuarios iniciado en el puerto "+port)
	log.Printf("usuarios listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
