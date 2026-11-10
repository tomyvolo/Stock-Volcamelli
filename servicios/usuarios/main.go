package main

import (
	"log"
	"os"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"usuarios/controllers"
	usersControllers "usuarios/controllers/users"
	"usuarios/database"
	"usuarios/logs"
	"usuarios/observability"
	usersRepo "usuarios/repositories/users"
	usersServices "usuarios/services/users"
)

const stage = "usuarios"

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}
	// Sin la clave no se puede firmar ningún token, así que no tiene sentido arrancar.
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("falta la variable de entorno JWT_SECRET")
	}
	db := database.ConnectDB()

	// Inyección de dependencias: repositorio -> servicio -> controlador.
	// Con CACHE_ENABLED=false el service usa directo MySQL, así medimos el mismo código con y sin caché.
	mysqlRepository := usersRepo.UsersMySQL{DB: db}
	var usersRepository usersRepo.UsersRepo = mysqlRepository
	if os.Getenv("CACHE_ENABLED") != "false" {
		memcachedAddr := os.Getenv("MEMCACHED_ADDR")
		if memcachedAddr == "" {
			memcachedAddr = "localhost:11211"
		}
		memcachedClient := memcache.New(memcachedAddr)
		// Si Memcached no responde rápido, preferimos ir a MySQL antes que hacer esperar la consulta.
		memcachedClient.Timeout = 100 * time.Millisecond
		usersRepository = usersRepo.UsersMemcached{Client: memcachedClient, NextRepo: mysqlRepository}
		log.Printf("caché de usuarios en Memcached (%s)", memcachedAddr)
	} else {
		log.Print("caché apagada: las consultas van directo a MySQL")
	}
	usersService := usersServices.UsersService{Repo: usersRepository}
	usersController := usersControllers.UsersController{Service: usersService, Logs: logStore}

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", controllers.Health)

	api := router.Group("/api")
	api.Use(observability.RecordMetrics, observability.ReadTrackID)

	api.POST("/auth/login", usersController.Login)
	// El gateway ya controló el token y que el rol sea DIRECTOR o ADMIN_SUCURSAL.
	api.POST("/create-users", usersController.Create)
	api.GET("/users", usersController.List)
	// El gateway solo deja pasar estas dos con el token de un director.
	api.PUT("/users/:userId", usersController.Update)
	api.DELETE("/users/:userId", usersController.Delete)

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
