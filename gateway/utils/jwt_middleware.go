package utils

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if header == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse(CodeUnauthorized, "token no provisto"))
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse(CodeUnauthorized, "formato de token inválido"))
			return
		}

		claims, err := ValidateJWT(parts[1])
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse(CodeUnauthorized, "token inválido o expirado"))
			return
		}

		ctx.Set("user_id", claims.UserID)
		ctx.Set("email", claims.Email)
		ctx.Set("role", claims.Role)
		if claims.BranchID != nil {
			ctx.Set("branch_id", *claims.BranchID)
		}

		ctx.Next()
	}
}

// RoleRequired deja pasar solo a los roles de la lista. Va siempre después de AuthRequired.
func RoleRequired(roles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userRole, exists := ctx.Get("role")
		if !exists {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse(CodeUnauthorized, "no autenticado"))
			return
		}

		if !slices.Contains(roles, userRole.(string)) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse(CodeForbidden, "no tenés permisos para esta acción"))
			return
		}

		ctx.Next()
	}
}
