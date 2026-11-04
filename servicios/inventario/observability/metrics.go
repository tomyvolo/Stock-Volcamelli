package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "requests_total",
	Help: "Cantidad de respuestas HTTP.",
}, []string{"route", "status"})

var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "request_duration_seconds",
	Help: "Duración de las respuestas HTTP en segundos.",
}, []string{"route", "status"})

// RecordMetrics mide una vez cada petición atendida por un controller.
func RecordMetrics(ctx *gin.Context) {
	startTime := time.Now()
	ctx.Next()

	route := ctx.FullPath()
	statusCode := strconv.Itoa(ctx.Writer.Status())

	httpRequests.WithLabelValues(route, statusCode).Inc()
	requestDuration.WithLabelValues(route, statusCode).Observe(time.Since(startTime).Seconds())
}
