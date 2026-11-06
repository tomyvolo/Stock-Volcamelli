package branches

import (
	"errors"
	"testing"

	"sucursales/models/branches"
	repo "sucursales/repositories/branches"
)

// newService arma el Service con el Mock en lugar de MySQL, con las sucursales 1 y 2.
func newService() BranchesService {
	mockRepo := repo.BranchesMock{
		MockDB: map[uint]branches.BranchModel{
			1: {ID: 1, Name: "Córdoba Centro", Address: "Av. Colón 1250, Córdoba"},
			2: {ID: 2, Name: "Nueva Córdoba", Address: "Bv. San Juan 480, Córdoba"},
		},
	}
	return BranchesService{Repo: mockRepo}
}

func TestGetBranches(t *testing.T) {
	service := newService()

	list, err := service.GetBranches()

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 {
		t.Errorf("Se esperaban las sucursales 1 y 2 en orden, pero se obtuvo: %+v", list)
	}
}

func TestGetBranch_Success(t *testing.T) {
	service := newService()

	branch, err := service.GetBranch(2)

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if branch.Name != "Nueva Córdoba" || branch.Address != "Bv. San Juan 480, Córdoba" {
		t.Errorf("No es la sucursal 2: %+v", branch)
	}
}

func TestGetBranch_NotFound(t *testing.T) {
	service := newService()

	_, err := service.GetBranch(99)

	if !errors.Is(err, ErrBranchNotFound) {
		t.Errorf("Se esperaba ErrBranchNotFound, pero se obtuvo: %v", err)
	}
}

func TestCreateBranch_Success(t *testing.T) {
	service := newService()

	branch, err := service.CreateBranch("  Villa Carlos Paz ", "Av. Libertad 320")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if branch.ID != 3 || branch.Name != "Villa Carlos Paz" {
		t.Errorf("Se esperaba la sucursal 3 sin espacios de más, pero se obtuvo: %+v", branch)
	}
	if _, err := service.GetBranch(3); err != nil {
		t.Errorf("La sucursal nueva no quedó guardada: %v", err)
	}
}

func TestCreateBranch_EmptyData(t *testing.T) {
	service := newService()

	_, err := service.CreateBranch("Sucursal sin dirección", "   ")

	if !errors.Is(err, ErrMissingData) {
		t.Errorf("Se esperaba ErrMissingData, pero se obtuvo: %v", err)
	}
}

func TestUpdateBranch_Success(t *testing.T) {
	service := newService()

	_, err := service.UpdateBranch(1, "Córdoba Centro", "Av. Colón 2000, Córdoba")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	branch, _ := service.GetBranch(1)
	if branch.Address != "Av. Colón 2000, Córdoba" {
		t.Errorf("La dirección no cambió: %+v", branch)
	}
}

func TestUpdateBranch_NotFound(t *testing.T) {
	service := newService()

	_, err := service.UpdateBranch(99, "Otra", "Otra dirección")

	if !errors.Is(err, ErrBranchNotFound) {
		t.Errorf("Se esperaba ErrBranchNotFound, pero se obtuvo: %v", err)
	}
}
