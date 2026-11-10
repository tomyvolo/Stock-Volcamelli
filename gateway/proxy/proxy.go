package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gateway/logs"
	"gateway/utils"
)

// Timeout es lo máximo que esperamos a un servicio antes de responder 503.
const Timeout = 2 * time.Second

// TransfersTimeout es lo que esperamos a Transferencias al crear o cancelar. Transferencias, a su vez,
// espera hasta 2 segundos a Inventario y a Sucursales: si esperáramos lo mismo, cortaríamos antes de que
// nos conteste que Inventario no respondió o que la transferencia quedó FALLIDA, y el usuario vería un
// 503 genérico (ADR-010).
const TransfersTimeout = 5 * time.Second

// AssistantTimeout es lo que esperamos al Asistente: un modelo puede tardar varios segundos y cada
// pregunta puede dar varias vueltas de herramientas (ADR-018).
const AssistantTimeout = 30 * time.Second

// Proxy reenvía los pedidos al servicio que corresponde.
type Proxy struct {
	Logs   *logs.Store
	Client http.Client
	// StreamClient es para Tiempo Real, cuya respuesta queda abierta: no tiene un timeout total,
	// pero igual espera como mucho 2 segundos a conectarse y a que lleguen los headers.
	StreamClient http.Client
	// TransfersClient es para crear y cancelar transferencias, que espera hasta 5 segundos.
	TransfersClient http.Client
	// AssistantClient es para el Asistente, que espera hasta 30 segundos.
	AssistantClient http.Client
}

func NewProxy(logStore *logs.Store) *Proxy {
	return &Proxy{
		Logs:   logStore,
		Client: http.Client{Timeout: Timeout},
		StreamClient: http.Client{Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: Timeout}).DialContext,
			ResponseHeaderTimeout: Timeout,
		}},
		TransfersClient: http.Client{Timeout: TransfersTimeout},
		AssistantClient: http.Client{Timeout: AssistantTimeout},
	}
}

// Forward manda el pedido a target (por ejemplo http://usuarios:8081) con la misma ruta y
// devuelve la respuesta del servicio tal cual. Al servicio le pasa el track_id y, si el
// pedido trae token, el id, el rol y la sucursal del usuario.
func (p *Proxy) Forward(service, target string) gin.HandlerFunc {
	return p.forward(service, target, &p.Client, false)
}

// TransferChange reenvía crear y cancelar una transferencia: igual que Forward, pero espera hasta 5 segundos.
func (p *Proxy) TransferChange(service, target string) gin.HandlerFunc {
	return p.forward(service, target, &p.TransfersClient, false)
}

// Assistant reenvía /api/assistant/* al Asistente. Espera hasta 30 segundos y es la única ruta a la que le
// pasamos el token: con ese token el servidor MCP llama a la API, así el asistente puede lo mismo que el
// usuario y nada más (ADR-018).
func (p *Proxy) Assistant(service, target string) gin.HandlerFunc {
	return p.forward(service, target, &p.AssistantClient, true)
}

func (p *Proxy) forward(service, target string, client *http.Client, passToken bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		trackID := ctx.GetString("track_id")

		request, err := http.NewRequestWithContext(ctx.Request.Context(), ctx.Request.Method, target+ctx.Request.URL.RequestURI(), ctx.Request.Body)
		if err != nil {
			p.Logs.Write(trackID, "gateway", "error", "no se pudo armar el pedido a "+service+": "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo reenviar el pedido"))
			return
		}
		setHeaders(ctx, request, trackID)
		if passToken {
			request.Header.Set("Authorization", ctx.GetHeader("Authorization"))
		}

		response, err := client.Do(request)
		if err != nil {
			p.unavailable(ctx, trackID, fmt.Sprintf("%s no respondió: %v", service, err))
			return
		}
		defer response.Body.Close()

		// NGINX responde 502 si no llegó a ninguna instancia de Inventario y 504 si tardaron demasiado, con su
		// página HTML. Un 502 en JSON es del propio servicio (la búsqueda cuando Solr no responde) y pasa tal cual.
		fromNginx := !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json")
		if fromNginx && (response.StatusCode == http.StatusBadGateway || response.StatusCode == http.StatusGatewayTimeout) {
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

// Stream reenvía GET /api/events a Tiempo Real. A diferencia de Forward, no espera la respuesta
// completa: le pasa al navegador cada mensaje apenas llega. La conexión se corta cuando el navegador
// se va, cuando Tiempo Real la cierra o cuando vence el token, y ahí el usuario vuelve a iniciar sesión.
func (p *Proxy) Stream(service, target string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		trackID := ctx.GetString("track_id")

		requestCtx := ctx.Request.Context()
		if expires, exists := ctx.Get("token_expires"); exists {
			var cancel context.CancelFunc
			requestCtx, cancel = context.WithDeadline(requestCtx, expires.(time.Time))
			defer cancel()
		}
		request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target+ctx.Request.URL.RequestURI(), nil)
		if err != nil {
			p.Logs.Write(trackID, "gateway", "error", "no se pudo armar el pedido a "+service+": "+err.Error())
			ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo reenviar el pedido"))
			return
		}
		setHeaders(ctx, request, trackID)

		response, err := p.StreamClient.Do(request)
		if err != nil {
			p.unavailable(ctx, trackID, fmt.Sprintf("%s no respondió: %v", service, err))
			return
		}
		defer response.Body.Close()

		ctx.Header("Content-Type", response.Header.Get("Content-Type"))
		ctx.Header("Cache-Control", "no-cache")
		ctx.Status(response.StatusCode)
		ctx.Writer.Flush()

		buffer := make([]byte, 4096)
		for {
			n, err := response.Body.Read(buffer)
			if n > 0 {
				ctx.Writer.Write(buffer[:n])
				ctx.Writer.Flush()
			}
			if err != nil {
				return
			}
		}
	}
}

// setHeaders arma los headers para el servicio de cero: los X-User-* que mande el cliente no llegan a los servicios.
func setHeaders(ctx *gin.Context, request *http.Request, trackID string) {
	request.Header.Set("Content-Type", ctx.GetHeader("Content-Type"))
	request.Header.Set("X-Track-ID", trackID)
	if userID, exists := ctx.Get("user_id"); exists {
		request.Header.Set("X-User-ID", fmt.Sprint(userID))
		request.Header.Set("X-User-Role", ctx.GetString("role"))
	}
	if branchID, exists := ctx.Get("branch_id"); exists {
		request.Header.Set("X-User-Branch", fmt.Sprint(branchID))
	}
}

// unavailable responde 503. Nunca respondemos stock cero ni una lista vacía, porque parecería un dato real.
func (p *Proxy) unavailable(ctx *gin.Context, trackID, reason string) {
	p.Logs.Write(trackID, "gateway", "error", reason)
	ctx.JSON(http.StatusServiceUnavailable, utils.ErrorResponse(utils.CodeServiceUnavailable, "El servicio no está disponible. Probá de nuevo en unos minutos."))
}
