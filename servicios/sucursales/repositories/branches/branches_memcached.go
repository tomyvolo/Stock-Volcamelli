package branches

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/bradfitz/gomemcache/memcache"

	"sucursales/models/branches"
	"sucursales/observability"
)

// Cuánto vive cada clave en Memcached. Un cambio en una sucursal se ve, como mucho, a los 60 segundos.
const cacheSeconds = 60

// BranchesMemcached busca primero en Memcached y, si no está, le pide el dato a NextRepo (MySQL)
// y lo guarda para la próxima. Si Memcached está caído, sigue con MySQL como si fuera un miss:
// la caché nunca hace fallar una consulta.
type BranchesMemcached struct {
	Client   *memcache.Client
	NextRepo BranchesRepo
}

func (repo BranchesMemcached) GetAll() ([]branches.BranchModel, error) {
	key := "SUCURSALES"

	var list []branches.BranchModel
	if repo.readCache(key, &list) {
		return list, nil
	}

	list, err := repo.NextRepo.GetAll()
	if err != nil {
		return nil, err
	}
	repo.writeCache(key, list)
	return list, nil
}

func (repo BranchesMemcached) GetByID(id uint) (branches.BranchModel, error) {
	key := fmt.Sprintf("SUCURSAL:%d", id)

	var branch branches.BranchModel
	if repo.readCache(key, &branch) {
		return branch, nil
	}

	branch, err := repo.NextRepo.GetByID(id)
	if err != nil {
		return branches.BranchModel{}, err
	}
	repo.writeCache(key, branch)
	return branch, nil
}

// Create y Update van directo a MySQL. No borramos las claves: el dato nuevo aparece cuando vencen.
func (repo BranchesMemcached) Create(branch *branches.BranchModel) error {
	return repo.NextRepo.Create(branch)
}

func (repo BranchesMemcached) Update(branch branches.BranchModel) error {
	return repo.NextRepo.Update(branch)
}

// readCache devuelve true si encontró la clave y pudo leerla.
func (repo BranchesMemcached) readCache(key string, value any) bool {
	cached, err := repo.Client.Get(key)
	if err == nil && json.Unmarshal(cached.Value, value) == nil {
		observability.CacheHits.Inc()
		return true
	}
	// Un miss es normal. Cualquier otro error es que Memcached no respondió.
	if err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
		observability.CacheErrors.Inc()
		log.Printf("memcached no respondió al leer %s, vamos a MySQL: %v", key, err)
	}
	observability.CacheMisses.Inc()
	return false
}

// writeCache guarda el valor como JSON. Si falla, la consulta ya tiene su respuesta, así que solo lo registra.
func (repo BranchesMemcached) writeCache(key string, value any) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return
	}
	err = repo.Client.Set(&memcache.Item{Key: key, Value: bytes, Expiration: cacheSeconds})
	if err != nil {
		observability.CacheErrors.Inc()
		log.Printf("memcached no respondió al guardar %s: %v", key, err)
	}
}
