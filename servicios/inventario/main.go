package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"inventario/controllers"
	productsControllers "inventario/controllers/products"
	reservationsControllers "inventario/controllers/reservations"
	stockControllers "inventario/controllers/stock"
	"inventario/database"
	"inventario/logs"
	"inventario/messaging"
	"inventario/observability"
	branchesRepo "inventario/repositories/branches"
	productsRepo "inventario/repositories/products"
	stockRepo "inventario/repositories/stock"
	productsServices "inventario/services/products"
	reservationsServices "inventario/services/reservations"
	stockServices "inventario/services/stock"
	"inventario/utils"
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

	sucursalesURL := os.Getenv("SUCURSALES_URL")
	if sucursalesURL == "" {
		sucursalesURL = "http://localhost:8083"
	}
	solrURL := os.Getenv("SOLR_URL")
	if solrURL == "" {
		solrURL = "http://localhost:8983/solr/productos"
	}
	rabbitURL := os.Getenv("RABBITMQ_URL")

	// Si no hay RabbitMQ, los productos se guardan igual y el evento queda avisado en el log.
	var publisher productsServices.Publisher = messaging.NullPublisher{}
	if rabbitURL != "" {
		publisher = messaging.RabbitMQPublisher{URL: rabbitURL, Logs: logStore}
	}

	// Inyección de dependencias: repositorio -> servicio -> controlador.
	productsRepository := productsRepo.ProductsMySQL{DB: db}
	stockRepository := stockRepo.StockMySQL{DB: db}
	stockChangesRepository := stockRepo.StockChangesMySQL{DB: db}
	// Como todas las llamadas entre servicios, la consulta a Sucursales espera como mucho 2 segundos.
	// Publicado en la nube no está Sucursales: las sucursales válidas son una lista fija.
	published := os.Getenv("INVENTARIO_API_KEY") != ""
	var branchesRepository branchesRepo.BranchesRepo = branchesRepo.BranchesHTTP{BaseURL: sucursalesURL, Client: &http.Client{Timeout: 2 * time.Second}}
	if published {
		branchesRepository = branchesRepo.BranchesList{IDs: []uint{1, 2, 3}}
	}
	searchRepository := productsRepo.SearchSolr{BaseURL: solrURL, Client: &http.Client{Timeout: 2 * time.Second}}

	productsService := productsServices.ProductsService{Repo: productsRepository, Search: searchRepository, Publisher: publisher}
	stockService := stockServices.StockService{
		Repo: stockRepository, ProductsRepo: productsRepository,
		Changes: stockChangesRepository, Branches: branchesRepository,
	}
	reservationsService := reservationsServices.ReservationsService{Repo: stockChangesRepository, ProductsRepo: productsRepository}

	productsController := productsControllers.ProductsController{Service: productsService, Logs: logStore}
	stockController := stockControllers.StockController{Service: stockService, Logs: logStore}
	reservationsController := reservationsControllers.ReservationsController{Service: reservationsService, Logs: logStore}

	router.Use(gin.Recovery())
	// Estas dos rutas quedan fuera del grupo medido para no contar las sondas.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", controllers.Health)

	api := router.Group("/api")
	api.Use(observability.RecordMetrics, observability.ReadTrackID, utils.APIKeyRequired())

	// Las 4 consultas del contrato (docs/contracts/openapi.yaml).
	api.GET("/products", productsController.List)
	api.GET("/products/:productId", productsController.GetByID)
	api.GET("/branches/:branchId/stock", stockController.GetByBranch)
	api.GET("/branches/:branchId/products/:productId/stock", stockController.GetByProduct)

	// Con API key, Inventario está publicado solo en la nube para el otro grupo, con su propia base: ahí no
	// hay gateway que controle roles, así que suman solo aumentar y reducir stock. Lo demás existe solo
	// detrás del gateway.
	if published {
		api.POST("/branches/:branchId/products/:productId/stock/increase", stockController.Increase)
		api.POST("/branches/:branchId/products/:productId/stock/decrease", stockController.Decrease)
	} else {
		log.Print("sin INVENTARIO_API_KEY: las consultas no piden API key, así que solo se puede usar detrás del gateway")

		// El gateway ya controló el rol: el director, o el admin de sucursal en los ajustes.
		// La búsqueda es solo para nuestro frontend: no está en el contrato.
		api.GET("/products/search", productsController.Search)
		api.GET("/branches/:branchId/products/:productId/movements", stockController.Movements)
		api.POST("/branches/:branchId/products/:productId/adjustments", stockController.Adjust)
		api.POST("/products", productsController.Create)
		api.PUT("/products/:productId", productsController.Update)

		// Reservas: solo las usa Transferencias, directo por el NGINX de Inventario y sin pasar por el gateway.
		internal := router.Group("/internal")
		internal.Use(observability.RecordMetrics, observability.ReadTrackID)
		internal.POST("/reservations", reservationsController.Reserve)
		internal.POST("/reservations/:transferId/release", reservationsController.Release)

		// Los eventos de Transferencias: confirman o liberan la reserva cuando el traslado termina.
		if rabbitURL != "" {
			transfersConsumer := messaging.TransfersConsumer{URL: rabbitURL, Service: reservationsService, Logs: logStore}
			transfersConsumer.Start()

			// El indexador corre en una sola instancia, la que tiene INDEXADOR=true.
			if os.Getenv("INDEXADOR") == "true" {
				productsIndexer := messaging.ProductsIndexer{URL: rabbitURL, Service: productsService, Logs: logStore}
				productsIndexer.Start()
			}
		} else {
			log.Print("sin RABBITMQ_URL: no se escuchan los eventos de Transferencias ni se actualiza la búsqueda")
		}
	}

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
