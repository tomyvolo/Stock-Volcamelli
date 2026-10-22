package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"inventario/controllers"
	productsControllers "inventario/controllers/products"
	stockControllers "inventario/controllers/stock"
	"inventario/database"
	"inventario/logs"
	"inventario/observability"
	productsRepo "inventario/repositories/products"
	stockRepo "inventario/repositories/stock"
	productsServices "inventario/services/products"
	stockServices "inventario/services/stock"
)

const stage = "inventario"

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}
	db := database.ConnectDB()

	// Inyección de dependencias: repositorio -> servicio -> controlador.
	productsRepository := productsRepo.ProductsMySQL{DB: db}
	stockRepository := stockRepo.StockMySQL{DB: db}

	productsService := productsServices.ProductsService{Repo: productsRepository}
	stockService := stockServices.StockService{Repo: stockRepository, ProductsRepo: productsRepository}

	productsController := productsControllers.ProductsController{Service: productsService, Logs: logStore}
	stockController := stockControllers.StockController{Service: stockService, Logs: logStore}

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", controllers.Health)

	api := router.Group("/api")
	api.Use(observability.RecordMetrics, observability.ReadTrackID)

	// Las 4 consultas del contrato (docs/contracts/openapi.yaml).
	api.GET("/products", productsController.List)
	api.GET("/products/:productId", productsController.GetByID)
	api.GET("/branches/:branchId/stock", stockController.GetByBranch)
	api.GET("/branches/:branchId/products/:productId/stock", stockController.GetByProduct)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	logStore.Write("", stage, "info", "inventario iniciado en el puerto "+port)
	log.Printf("inventario listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
