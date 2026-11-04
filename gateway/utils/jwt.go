package utils

import (
	"errors"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// Claims es lo que lleva el token. Es la misma estructura con la que lo firma Usuarios.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	BranchID *uint  `json:"branch_id"` // nil para DIRECTOR
	jwt.RegisteredClaims
}

func ValidateJWT(tokenString string) (*Claims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET no configurado")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("método de firma inesperado")
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("token inválido")
	}

	return claims, nil
}
