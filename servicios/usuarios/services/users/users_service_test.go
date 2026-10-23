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

func TestCreateUser_DirectorCreatesAdmin(t *testing.T) {
	service := newService(t)
	branch := uint(2)

	user, err := service.CreateUser("DIRECTOR", nil,
		users.UserModel{Name: "Admin 2", Email: "admin2@volcamelli.com", Role: "ADMIN_SUCURSAL", BranchID: &branch}, "clave-nueva")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if user.ID == 0 {
		t.Errorf("Se esperaba que el usuario tuviera un ID")
	}
	// La contraseña se guarda hasheada, nunca en texto.
	if user.PasswordHash == "clave-nueva" || !utils.CheckPassword("clave-nueva", user.PasswordHash) {
		t.Errorf("La contraseña no quedó hasheada con bcrypt")
	}
}

func TestCreateUser_AdminCreatesSellerOfOwnBranch(t *testing.T) {
	service := newService(t)
	branch := uint(1)

	_, err := service.CreateUser("ADMIN_SUCURSAL", &branch,
		users.UserModel{Name: "Vendedor 2", Email: "vendedor2@volcamelli.com", Role: "VENDEDOR", BranchID: &branch}, "clave-nueva")

	if err != nil {
		t.Errorf("No se esperaba un error, pero falló: %v", err)
	}
}

func TestCreateUser_AdminCannotCreateAdmin(t *testing.T) {
	service := newService(t)
	branch := uint(1)

	_, err := service.CreateUser("ADMIN_SUCURSAL", &branch,
		users.UserModel{Name: "Otro admin", Email: "admin3@volcamelli.com", Role: "ADMIN_SUCURSAL", BranchID: &branch}, "clave-nueva")

	if !errors.Is(err, ErrNotAllowed) {
		t.Errorf("Se esperaba ErrNotAllowed, pero se obtuvo: %v", err)
	}
}

func TestCreateUser_AdminCannotCreateInOtherBranch(t *testing.T) {
	service := newService(t)
	adminBranch, otherBranch := uint(1), uint(2)

	_, err := service.CreateUser("ADMIN_SUCURSAL", &adminBranch,
		users.UserModel{Name: "Vendedor", Email: "vendedor3@volcamelli.com", Role: "VENDEDOR", BranchID: &otherBranch}, "clave-nueva")

	if !errors.Is(err, ErrNotAllowed) {
		t.Errorf("Se esperaba ErrNotAllowed, pero se obtuvo: %v", err)
	}
}

func TestCreateUser_EmailTaken(t *testing.T) {
	service := newService(t)
	branch := uint(1)

	_, err := service.CreateUser("DIRECTOR", nil,
		users.UserModel{Name: "Repetido", Email: "vendedor@volcamelli.com", Role: "VENDEDOR", BranchID: &branch}, "clave-nueva")

	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("Se esperaba ErrEmailTaken, pero se obtuvo: %v", err)
	}
}

func TestCreateUser_SellerNeedsBranch(t *testing.T) {
	service := newService(t)

	_, err := service.CreateUser("DIRECTOR", nil,
		users.UserModel{Name: "Sin sucursal", Email: "vendedor4@volcamelli.com", Role: "VENDEDOR"}, "clave-nueva")

	if !errors.Is(err, ErrBranchRequired) {
		t.Errorf("Se esperaba ErrBranchRequired, pero se obtuvo: %v", err)
	}
}
