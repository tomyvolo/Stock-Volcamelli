package branches

import (
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"sucursales/models/branches"
)

// Si Memcached no responde, la consulta tiene que salir igual desde MySQL, sin error.
// El puerto 1 no tiene a nadie escuchando, así que simula un Memcached caído.
func TestMemcachedDown_FallsBackToMySQL(t *testing.T) {
	calls := 0
	mysqlMock := BranchesMock{
		MockDB: map[uint]branches.BranchModel{
			1: {ID: 1, Name: "Córdoba Centro", Address: "Av. Colón 1250, Córdoba"},
		},
		Calls: &calls,
	}
	client := memcache.New("127.0.0.1:1")
	client.Timeout = 100 * time.Millisecond
	repo := BranchesMemcached{Client: client, NextRepo: mysqlMock}

	branch, err := repo.GetByID(1)
	if err != nil {
		t.Fatalf("Con Memcached caído no se esperaba un error, pero falló: %v", err)
	}
	if branch.Name != "Córdoba Centro" {
		t.Errorf("No es la sucursal 1: %+v", branch)
	}

	list, err := repo.GetAll()
	if err != nil || len(list) != 1 {
		t.Fatalf("Con Memcached caído se esperaba la lista desde MySQL, pero se obtuvo %+v, %v", list, err)
	}
	if calls != 2 {
		t.Errorf("Las dos consultas tenían que ir a MySQL, pero fueron %d", calls)
	}
}
