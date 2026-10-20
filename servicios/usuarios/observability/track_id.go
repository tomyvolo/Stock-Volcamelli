package observability

import "github.com/gin-gonic/gin"

// ReadTrackID toma el TrackID que generó el gateway y lo deja en el contexto.
// Los servicios nunca generan uno propio: si llamamos a otro servicio, lo pasamos igual en X-Track-ID.
func ReadTrackID(ctx *gin.Context) {
	ctx.Set("track_id", ctx.GetHeader("X-Track-ID"))
	ctx.Next()
}
