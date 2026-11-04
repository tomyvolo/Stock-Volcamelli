package observability

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// ReadTrackID toma el TrackID que generó el gateway y lo deja en el contexto.
// Si el pedido llega directo, como cuando Inventario está publicado solo, crea uno igual que el gateway.
// En los dos casos lo devuelve en X-Track-ID, para que quien consulta pueda buscar la traza.
func ReadTrackID(ctx *gin.Context) {
	trackID := ctx.GetHeader("X-Track-ID")
	if len(trackID) != 32 {
		var randomBytes [16]byte
		if _, err := rand.Read(randomBytes[:]); err == nil {
			trackID = hex.EncodeToString(randomBytes[:])
		}
	}
	ctx.Set("track_id", trackID)
	ctx.Header("X-Track-ID", trackID)
	ctx.Next()
}
