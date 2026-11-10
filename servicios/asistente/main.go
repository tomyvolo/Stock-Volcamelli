package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"asistente/chat"
	"asistente/logs"
	"asistente/model"
	"asistente/tools"
)

const stage = "asistente"

// Lo máximo que puede tardar una pregunta. El gateway espera 30 segundos en esta ruta (ADR-018).
const requestTimeout = 28 * time.Second

var replies = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "assistant_replies_total",
	Help: "Respuestas del asistente: con texto, con tarjeta de confirmación o sin poder responder.",
}, []string{"route", "result"})

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

type chatRequest struct {
	Messages []chat.Message `json:"messages" binding:"required"`
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	logStore, err := logs.Connect()
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de logs: %v", err)
	}
	// Sin clave el asistente arranca igual, pero cada pregunta responde que no está disponible.
	if os.Getenv("MODEL_API_KEY") == "" {
		logStore.Write("", stage, "warn", "falta MODEL_API_KEY: el asistente va a responder que no está disponible")
	}

	service := chat.Service{
		Model: model.OpenAI{
			BaseURL: env("MODEL_URL", "https://api.groq.com/openai/v1"),
			Model:   env("MODEL_NAME", "openai/gpt-oss-120b"),
			APIKey:  os.Getenv("MODEL_API_KEY"),
			HTTP:    &http.Client{Timeout: model.Timeout},
		},
		Tools: tools.Client{Endpoint: env("MCP_URL", "http://localhost:8086/mcp")},
		Logs:  logStore,
	}

	router.Use(gin.Recovery())
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// El gateway le pasa el header Authorization solo a estas rutas: con ese token el servidor MCP llama
	// a la API, así el asistente puede lo mismo que el usuario y nada más.
	router.POST("/api/assistant/chat", func(ctx *gin.Context) {
		var request chatRequest
		if err := ctx.ShouldBindJSON(&request); err != nil || len(request.Messages) == 0 ||
			request.Messages[len(request.Messages)-1].Role != "user" {
			ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", "Hace falta messages, con el último mensaje del usuario."))
			return
		}
		respond(ctx, "chat", func(c context.Context, authorization, trackID string) (chat.Reply, error) {
			return service.Chat(c, authorization, trackID, request.Messages)
		})
	})
	router.POST("/api/assistant/confirm", func(ctx *gin.Context) {
		var transfer chat.Transfer
		if err := ctx.ShouldBindJSON(&transfer); err != nil {
			ctx.JSON(http.StatusBadRequest, errorJSON("INVALID_REQUEST", "Hacen falta productId, quantity, originBranchId y destinationBranchId."))
			return
		}
		respond(ctx, "confirm", func(c context.Context, authorization, trackID string) (chat.Reply, error) {
			return service.Confirm(c, authorization, trackID, transfer)
		})
	})

	port := env("PORT", "8087")
	logStore.Write("", stage, "info", "asistente iniciado en el puerto "+port)
	log.Printf("asistente listo en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// respond corre la pregunta con el token y el TrackID que manda el gateway, y responde.
func respond(ctx *gin.Context, route string, run func(c context.Context, authorization, trackID string) (chat.Reply, error)) {
	authorization := ctx.GetHeader("Authorization")
	if authorization == "" {
		ctx.JSON(http.StatusUnauthorized, errorJSON("UNAUTHORIZED", "Falta el token del usuario."))
		return
	}
	c, cancel := context.WithTimeout(ctx.Request.Context(), requestTimeout)
	defer cancel()

	reply, err := run(c, authorization, ctx.GetHeader("X-Track-ID"))
	switch {
	case errors.Is(err, chat.ErrUnavailable):
		replies.WithLabelValues(route, "unavailable").Inc()
		ctx.JSON(http.StatusServiceUnavailable, errorJSON("ASSISTANT_UNAVAILABLE", "El asistente no está disponible en este momento. Probá de nuevo en unos minutos."))
	case err != nil:
		replies.WithLabelValues(route, "error").Inc()
		ctx.JSON(http.StatusInternalServerError, errorJSON("INTERNAL_ERROR", "Ocurrió un error inesperado."))
	case reply.Confirmation != nil:
		replies.WithLabelValues(route, "confirmation").Inc()
		ctx.JSON(http.StatusOK, reply)
	default:
		replies.WithLabelValues(route, "ok").Inc()
		ctx.JSON(http.StatusOK, reply)
	}
}

func errorJSON(code, message string) gin.H {
	return gin.H{"code": code, "message": message}
}
