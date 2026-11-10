package users

import (
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"usuarios/models/users"
)

// Si Memcached no responde, la lista tiene que salir igual desde MySQL, sin error.
// El puerto 1 no tiene a nadie escuchando, así que simula un Memcached caído.
func TestMemcachedDown_FallsBackToMySQL(t *testing.T) {
	calls := 0
	mysqlMock := UsersMock{
		MockDB: map[string]users.UserModel{
			"director@volcamelli.com": {ID: 1, Name: "Director", Email: "director@volcamelli.com", PasswordHash: "hash", Role: "DIRECTOR"},
		},
		Calls: &calls,
	}
	client := memcache.New("127.0.0.1:1")
	client.Timeout = 100 * time.Millisecond
	repo := UsersMemcached{Client: client, NextRepo: mysqlMock}

	list, err := repo.GetAll()

	if err != nil {
		t.Fatalf("Con Memcached caído no se esperaba un error, pero falló: %v", err)
	}
	if len(list) != 1 || list[0].Email != "director@volcamelli.com" || calls != 1 {
		t.Errorf("Se esperaba la lista desde MySQL, pero se obtuvo %+v con %d consultas", list, calls)
	}
}
