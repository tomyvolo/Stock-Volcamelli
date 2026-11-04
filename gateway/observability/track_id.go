package observability

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"gateway/utils"
)

// AddTrackID asigna un ID distinto a cada petición del grupo /api.
func AddTrackID(ctx *gin.Context) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, utils.ErrorResponse(utils.CodeInternal, "no se pudo crear el track_id"))
		return
	}
	trackID := hex.EncodeToString(randomBytes[:])
	ctx.Set("track_id", trackID)
	ctx.Header("X-Track-ID", trackID)
	ctx.Next()
}
