package users

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/bradfitz/gomemcache/memcache"

	"usuarios/models/users"
	"usuarios/observability"
)

// Cuánto vive la lista en Memcached. Un usuario nuevo aparece, como mucho, a los 60 segundos.
const cacheSeconds = 60

// UsersMemcached guarda en Memcached la lista de usuarios y, si no la tiene, se la pide a NextRepo (MySQL).
// Si Memcached está caído, sigue con MySQL como si fuera un miss: la caché nunca hace fallar una consulta.
// El login y el alta van siempre directo a MySQL.
type UsersMemcached struct {
	Client   *memcache.Client
	NextRepo UsersRepo
}

func (repo UsersMemcached) GetUserByEmail(email string) (users.UserModel, error) {
	return repo.NextRepo.GetUserByEmail(email)
}

// No borramos la clave al crear: el usuario nuevo aparece en la lista cuando vence.
func (repo UsersMemcached) CreateUser(user *users.UserModel) error {
	return repo.NextRepo.CreateUser(user)
}

func (repo UsersMemcached) GetUserByID(id uint) (users.UserModel, error) {
	return repo.NextRepo.GetUserByID(id)
}

// Igual que en el alta, la lista de la caché muestra el cambio cuando vence.
func (repo UsersMemcached) UpdateRoleAndBranch(id uint, role string, branchID *uint) error {
	return repo.NextRepo.UpdateRoleAndBranch(id, role, branchID)
}

// Igual que en el alta y los cambios, el usuario borrado sale de la lista de la caché cuando vence.
func (repo UsersMemcached) DeleteUser(id uint) error {
	return repo.NextRepo.DeleteUser(id)
}

func (repo UsersMemcached) GetAll() ([]users.UserSummary, error) {
	key := "USUARIOS"

	cached, err := repo.Client.Get(key)
	if err == nil {
		var list []users.UserSummary
		if json.Unmarshal(cached.Value, &list) == nil {
			observability.CacheHits.Inc()
			return list, nil
		}
	}
	// Un miss es normal. Cualquier otro error es que Memcached no respondió.
	if err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
		observability.CacheErrors.Inc()
		log.Printf("memcached no respondió al leer %s, vamos a MySQL: %v", key, err)
	}
	observability.CacheMisses.Inc()

	list, err := repo.NextRepo.GetAll()
	if err != nil {
		return nil, err
	}

	// Si no se puede guardar, la consulta ya tiene su respuesta, así que solo lo registramos.
	bytes, _ := json.Marshal(list)
	if err := repo.Client.Set(&memcache.Item{Key: key, Value: bytes, Expiration: cacheSeconds}); err != nil {
		observability.CacheErrors.Inc()
		log.Printf("memcached no respondió al guardar %s: %v", key, err)
	}
	return list, nil
}
