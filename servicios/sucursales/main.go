package main

import (
	"log"
	"os"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"sucursales/controllers"
	branchesControllers "sucursales/controllers/branches"
	"sucursales/database"
	"sucursales/logs"
	"sucursales/observability"
	branchesRepo "sucursales/repositories/branches"
	branchesServices "sucursales/services/branches"
)

const stage = "sucursales"

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}
	db := database.ConnectDB()

	// Inyección de dependencias: repositorio -> servicio -> controlador.
	// Con CACHE_ENABLED=false el service usa directo MySQL, así medimos el mismo código con y sin caché.
	mysqlRepository := branchesRepo.BranchesMySQL{DB: db}
	var branchesRepository branchesRepo.BranchesRepo = mysqlRepository
	if os.Getenv("CACHE_ENABLED") != "false" {
		memcachedAddr := os.Getenv("MEMCACHED_ADDR")
		if memcachedAddr == "" {
			memcachedAddr = "localhost:11211"
		}
		memcachedClient := memcache.New(memcachedAddr)
		// Si Memcached no responde rápido, preferimos ir a MySQL antes que hacer esperar la consulta.
		memcachedClient.Timeout = 100 * time.Millisecond
		branchesRepository = branchesRepo.BranchesMemcached{Client: memcachedClient, NextRepo: mysqlRepository}
		log.Printf("caché de sucursales en Memcached (%s)", memcachedAddr)
	} else {
		log.Print("caché apagada: las consultas van directo a MySQL")
	}

	branchesService := branchesServices.BranchesService{Repo: branchesRepository}
	branchesController := branchesControllers.BranchesController{Service: branchesService, Logs: logStore}

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", controllers.Health)

	api := router.Group("/api")
	api.Use(observability.RecordMetrics, observability.ReadTrackID)

	api.GET("/branches", branchesController.List)
	api.GET("/branches/:branchId", branchesController.GetByID)
	// El gateway solo deja pasar estas dos con el token de un director.
	api.POST("/branches", branchesController.Create)
	api.PUT("/branches/:branchId", branchesController.Update)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	logStore.Write("", stage, "info", "sucursales iniciado en el puerto "+port)
	log.Printf("sucursales listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
