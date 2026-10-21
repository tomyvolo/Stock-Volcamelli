package observability

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"gateway/logs"
)

// LogRequest escribe un log al recibir cada pedido y otro al responderlo, con el mismo track_id.
// Va después de AddTrackID.
func LogRequest(logStore *logs.Store) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startTime := time.Now()
		trackID := ctx.GetString("track_id")
		logStore.Write(trackID, "gateway", "info", "pedido recibido "+ctx.Request.Method+" "+ctx.Request.URL.Path)

		ctx.Next()

		status := ctx.Writer.Status()
		level := "info"
		if status >= 500 {
			level = "error"
		} else if status >= 400 {
			level = "warn"
		}
		logStore.Write(trackID, "gateway", level, fmt.Sprintf("respuesta %d (%d ms)", status, time.Since(startTime).Milliseconds()))
	}
}
