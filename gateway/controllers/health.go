package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health permite comprobar que el proceso responde; también sirve al healthcheck de Compose.
func Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
