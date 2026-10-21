package proxy

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"gateway/logs"
	"gateway/utils"
)

// Timeout es lo máximo que esperamos a un servicio antes de responder 503.
const Timeout = 2 * time.Second

// Proxy reenvía los pedidos al servicio que corresponde.
type Proxy struct {
	Logs   *logs.Store
	Client http.Client
}

func NewProxy(logStore *logs.Store) *Proxy {
	return &Proxy{Logs: logStore, Client: http.Client{Timeout: Timeout}}
}

// Forward manda el pedido a target (por ejemplo http://usuarios:8081) con la misma ruta y
// devuelve la respuesta del servicio tal cual. Al servicio le pasa el track_id y, si el
// pedido trae token, el id, el rol y la sucursal del usuario.
func (p *Proxy) Forward(service, target string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		trackID := ctx.GetString("track_id")

		request, err := http.NewRequestWithContext(ctx.Request.Context(), ctx.Request.Method, target+ctx.Request.URL.RequestURI(), ctx.Request.Body)
		if err != nil {
			p.Logs.Write(trackID, "gateway", "error", "no se pudo armar el pedido a "+service+": "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo reenviar el pedido"))
			return
		}
		// Armamos los headers de cero: los X-User-* que mande el cliente no llegan a los servicios.
		request.Header.Set("Content-Type", ctx.GetHeader("Content-Type"))
		request.Header.Set("X-Track-ID", trackID)
		if userID, exists := ctx.Get("user_id"); exists {
			request.Header.Set("X-User-ID", fmt.Sprint(userID))
			request.Header.Set("X-User-Role", ctx.GetString("role"))
		}
		if branchID, exists := ctx.Get("branch_id"); exists {
			request.Header.Set("X-User-Branch", fmt.Sprint(branchID))
		}

		response, err := p.Client.Do(request)
		if err != nil {
			p.unavailable(ctx, trackID, fmt.Sprintf("%s no respondió: %v", service, err))
			return
		}
		defer response.Body.Close()

		// NGINX responde 502 si no llegó a ninguna instancia de Inventario y 504 si tardaron demasiado.
		if response.StatusCode == http.StatusBadGateway || response.StatusCode == http.StatusGatewayTimeout {
			p.unavailable(ctx, trackID, fmt.Sprintf("%s no disponible: NGINX respondió %d", service, response.StatusCode))
			return
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			p.unavailable(ctx, trackID, fmt.Sprintf("%s cortó la respuesta: %v", service, err))
			return
		}
		ctx.Data(response.StatusCode, response.Header.Get("Content-Type"), body)
	}
}

// unavailable responde 503. Nunca respondemos stock cero ni una lista vacía, porque parecería un dato real.
func (p *Proxy) unavailable(ctx *gin.Context, trackID, reason string) {
	p.Logs.Write(trackID, "gateway", "error", reason)
	ctx.JSON(http.StatusServiceUnavailable, utils.ErrorResponse(utils.CodeServiceUnavailable, "El servicio no está disponible. Probá de nuevo en unos minutos."))
}
