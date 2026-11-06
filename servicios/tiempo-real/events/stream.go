package events

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// KeepAlive es cada cuánto mandamos un comentario vacío para que la conexión no se cierre por inactividad.
const KeepAlive = 25 * time.Second

// Stream atiende GET /api/events con Server-Sent Events: deja la respuesta abierta y le escribe
// al navegador cada aviso apenas llega. Termina cuando el navegador (o el gateway) corta la conexión,
// o cuando el Hub lo desconecta por no leer.
func Stream(hub *Hub, keepAlive time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ch := hub.Subscribe()
		defer hub.Unsubscribe(ch)

		ctx.Header("Content-Type", "text/event-stream")
		ctx.Header("Cache-Control", "no-cache")
		ctx.Status(200)
		// Mandamos los headers ya, así el gateway sabe enseguida que Tiempo Real respondió.
		ctx.Writer.Flush()

		ticker := time.NewTicker(keepAlive)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Request.Context().Done():
				return
			case event, open := <-ch:
				if !open {
					return
				}
				fmt.Fprintf(ctx.Writer, "event: %s\ndata: %s\n\n", event.Type, event.Data)
				ctx.Writer.Flush()
			case <-ticker.C:
				fmt.Fprint(ctx.Writer, ":\n\n")
				ctx.Writer.Flush()
			}
		}
	}
}
