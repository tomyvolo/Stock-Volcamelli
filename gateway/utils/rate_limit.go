package utils

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CodeTooManyRequests es el código de error cuando una IP pasa el límite de pedidos.
const CodeTooManyRequests = "TOO_MANY_REQUESTS"

// RateLimit deja pasar hasta limit pedidos por minuto de cada IP; los que sobran reciben 429 sin llegar
// a ningún servicio. Cuenta por minuto del reloj: al empezar un minuto nuevo, todos los contadores
// vuelven a cero, así el mapa nunca crece más que las IPs de un minuto. Con limit 0 no limita (lo
// usamos en las pruebas de carga, que salen todas de la misma IP).
func RateLimit(limit int) gin.HandlerFunc {
	if limit <= 0 {
		return func(ctx *gin.Context) { ctx.Next() }
	}

	var mu sync.Mutex
	minute := time.Now().Truncate(time.Minute)
	counts := map[string]int{}

	return func(ctx *gin.Context) {
		now := time.Now()
		ip := ctx.ClientIP()

		mu.Lock()
		if current := now.Truncate(time.Minute); !current.Equal(minute) {
			minute = current
			counts = map[string]int{}
		}
		counts[ip]++
		exceeded := counts[ip] > limit
		nextMinute := minute.Add(time.Minute)
		mu.Unlock()

		if exceeded {
			wait := int(nextMinute.Sub(now).Seconds()) + 1
			ctx.Header("Retry-After", strconv.Itoa(wait))
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, ErrorResponse(CodeTooManyRequests,
				"demasiados pedidos, probá de nuevo en "+strconv.Itoa(wait)+" segundos"))
			return
		}
		ctx.Next()
	}
}
