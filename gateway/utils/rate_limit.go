package utils

import (
	"net"
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
//
// La IP es la de la conexión. Solo si el pedido viene de un proxy de confianza (el NGINX del frontend)
// se toma la del encabezado X-Real-IP, que ese NGINX pisa siempre con la IP real del navegador. Los
// encabezados que manda cualquier otro cliente se ignoran: si no, cambiando X-Forwarded-For en cada
// pedido se salteaba el límite.
func RateLimit(limit int, trustedProxy func(ip string) bool) gin.HandlerFunc {
	if limit <= 0 {
		return func(ctx *gin.Context) { ctx.Next() }
	}

	var mu sync.Mutex
	minute := time.Now().Truncate(time.Minute)
	counts := map[string]int{}

	return func(ctx *gin.Context) {
		now := time.Now()
		ip := clientIP(ctx, trustedProxy)

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

func clientIP(ctx *gin.Context, trustedProxy func(ip string) bool) string {
	ip := ctx.RemoteIP()
	if trustedProxy != nil && trustedProxy(ip) {
		if real := net.ParseIP(ctx.GetHeader("X-Real-IP")); real != nil {
			return real.String()
		}
	}
	return ip
}

// TrustedHost reconoce al proxy de confianza por su nombre en la red de Docker (por ejemplo frontend),
// porque su IP la asigna Docker al levantarlo. La vuelve a averiguar cada 10 segundos, por si el
// contenedor se recreó con otra. Con host vacío no hay proxy de confianza.
func TrustedHost(host string) func(ip string) bool {
	if host == "" {
		return nil
	}
	var mu sync.Mutex
	var ips map[string]bool
	var resolvedAt time.Time

	return func(ip string) bool {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(resolvedAt) > 10*time.Second {
			resolvedAt = time.Now()
			if addresses, err := net.LookupHost(host); err == nil {
				ips = map[string]bool{}
				for _, address := range addresses {
					ips[address] = true
				}
			}
		}
		return ips[ip]
	}
}
