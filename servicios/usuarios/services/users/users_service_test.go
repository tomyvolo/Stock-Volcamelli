package users

import (
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"usuarios/models/users"
	repo "usuarios/repositories/users"
	"usuarios/utils"
)

const testSecret = "clave-solo-para-los-tests"

// newService arma el Service con el Mock en lugar de MySQL, con un vendedor de la sucursal 1.
func newService(t *testing.T) UsersService {
	t.Setenv("JWT_SECRET", testSecret)

	hash, err := utils.HashPassword("clave-correcta")
	if err != nil {
		t.Fatalf("No se pudo generar el hash: %v", err)
	}
	branch := uint(1)
	mockRepo := repo.UsersMock{
		MockDB: map[string]users.UserModel{
			"vendedor@volcamelli.com": {ID: 3, Name: "Vendedor", Email: "vendedor@volcamelli.com", PasswordHash: hash, Role: "VENDEDOR", BranchID: &branch},
		},
	}
	return UsersService{Repo: mockRepo}
}

func TestLogin_Success(t *testing.T) {
	service := newService(t)

	token, user, err := service.Login("vendedor@volcamelli.com", "clave-correcta")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if user.ID != 3 {
		t.Errorf("Se esperaba el usuario 3, pero se obtuvo: %d", user.ID)
	}

	// El token tiene que llevar el id, el rol y la sucursal del usuario.
	claims := &utils.Claims{}
	_, err = jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("El token no es válido: %v", err)
	}
	if claims.UserID != 3 || claims.Role != "VENDEDOR" || claims.BranchID == nil || *claims.BranchID != 1 {
		t.Errorf("El token no tiene los datos del usuario: %+v", claims)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	service := newService(t)

	_, _, err := service.Login("vendedor@volcamelli.com", "otra-clave")

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Se esperaba ErrInvalidCredentials, pero se obtuvo: %v", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	service := newService(t)

	_, _, err := service.Login("nadie@volcamelli.com", "clave-correcta")

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Se esperaba ErrInvalidCredentials, pero se obtuvo: %v", err)
	}
}
