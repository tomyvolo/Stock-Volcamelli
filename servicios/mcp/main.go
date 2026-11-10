package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"mcp/gateway"
	"mcp/logs"
	"mcp/tools"
)

const stage = "mcp"

var toolCalls = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "tool_calls_total",
	Help: "Llamadas a cada herramienta, por resultado.",
}, []string{"tool", "result"})

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8080"
	}

	server := tools.NewServer(tools.Toolbox{
		Gateway: gateway.Client{BaseURL: gatewayURL, HTTP: &http.Client{Timeout: gateway.Timeout}},
		Logs:    logStore,
		Count:   func(tool, result string) { toolCalls.WithLabelValues(tool, result).Inc() },
	})
	// Sin sesión: cada pedido trae el token del usuario y nada queda guardado entre pedidos.
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})

	router.Use(gin.Recovery())
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/mcp", gin.WrapH(handler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}
	logStore.Write("", stage, "info", "servidor MCP iniciado en el puerto "+port)
	log.Printf("servidor MCP listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
