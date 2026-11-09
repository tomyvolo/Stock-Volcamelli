// Package sucursales le pide a Sucursales el nombre y la dirección de una sucursal, que Rumbo necesita.
package sucursales

import (
	"encoding/json"
	"fmt"
	"net/http"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Sucursales implementa puertos.Sucursales. El Client tiene que tener el timeout de 2 segundos.
type Sucursales struct {
	BaseURL string
	Client  *http.Client
}

func (s Sucursales) Obtener(sucursalID uint, trackID string) (dominio.Ubicacion, error) {
	pedido, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/branches/%d", s.BaseURL, sucursalID), nil)
	if err != nil {
		return dominio.Ubicacion{}, err
	}
	pedido.Header.Set("X-Track-ID", trackID)

	respuesta, err := s.Client.Do(pedido)
	if err != nil {
		return dominio.Ubicacion{}, fmt.Errorf("%w: %v", puertos.ErrNoDisponible, err)
	}
	defer respuesta.Body.Close()

	switch respuesta.StatusCode {
	case http.StatusOK:
		var sucursal struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		}
		if err := json.NewDecoder(respuesta.Body).Decode(&sucursal); err != nil {
			return dominio.Ubicacion{}, fmt.Errorf("%w: respuesta ilegible de Sucursales: %v", puertos.ErrNoDisponible, err)
		}
		return dominio.Ubicacion{Nombre: sucursal.Name, Direccion: sucursal.Address}, nil
	case http.StatusNotFound:
		return dominio.Ubicacion{}, puertos.ErrNoExiste
	default:
		return dominio.Ubicacion{}, fmt.Errorf("%w: Sucursales respondió %d", puertos.ErrNoDisponible, respuesta.StatusCode)
	}
}
