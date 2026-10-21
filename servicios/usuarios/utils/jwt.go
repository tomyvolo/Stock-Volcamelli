package utils

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims es lo que lleva el token. El gateway tiene la misma estructura para leerlo.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	BranchID *uint  `json:"branch_id"` // nil para DIRECTOR e INTEGRACION
	jwt.RegisteredClaims
}

// TokenHours es cuántas horas dura el token. Por defecto 1, como dice el contrato.
func TokenHours() int {
	hours, err := strconv.Atoi(os.Getenv("JWT_EXPIRES_HOURS"))
	if err != nil || hours <= 0 {
		hours = 1
	}
	return hours
}

func GenerateJWT(userID uint, email, role string, branchID *uint) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET no configurado")
	}

	claims := Claims{
		UserID:   userID,
		Email:    email,
		Role:     role,
		BranchID: branchID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(TokenHours()) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
