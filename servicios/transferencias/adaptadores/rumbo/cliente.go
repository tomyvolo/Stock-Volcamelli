// Package rumbo es el adaptador del Transporte para Rumbo, el servicio de traslados de otro grupo.
// Es el único lugar que conoce su contrato (docs/contracts/rumbo/traslados-v2.yaml): si Rumbo
// cambia, solo se toca este paquete.
package rumbo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"transferencias/puertos"
)

// Cliente implementa puertos.Transporte contra la API de Rumbo.
type Cliente struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
	Registro     puertos.Registro

	mu         sync.Mutex
	token      string
	tokenVence time.Time
}

// problema es el formato de error de Rumbo (Problem Details).
type problema struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Codigo string `json:"codigo"`
}

func (p problema) motivo() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

// obtenerToken devuelve el token guardado o pide uno nuevo si falta poco para que venza.
func (c *Cliente) obtenerToken(renovar bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !renovar && c.token != "" && time.Until(c.tokenVence) > time.Minute {
		return c.token, nil
	}

	cuerpo, _ := json.Marshal(map[string]string{"client_id": c.ClientID, "client_secret": c.ClientSecret})
	respuesta, err := c.HTTP.Post(c.BaseURL+"/v2/auth/token", "application/json", bytes.NewReader(cuerpo))
	if err != nil {
		return "", fmt.Errorf("%w: token: %v", puertos.ErrNoDisponible, err)
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: Rumbo respondió %d al pedir el token", puertos.ErrNoDisponible, respuesta.StatusCode)
	}

	var token struct {
		AccessToken      string `json:"access_token"`
		ExpiraEnSegundos int    `json:"expira_en_segundos"`
	}
	if err := json.NewDecoder(respuesta.Body).Decode(&token); err != nil {
		return "", fmt.Errorf("%w: token ilegible: %v", puertos.ErrNoDisponible, err)
	}
	c.token = token.AccessToken
	c.tokenVence = time.Now().Add(time.Duration(token.ExpiraEnSegundos) * time.Second)
	return c.token, nil
}

// pedir manda un pedido con el token. Si Rumbo responde 401, pide un token nuevo y reintenta una vez.
// Devuelve el cuerpo y el código; los errores de red y los 429 y 503 se devuelven como ErrNoDisponible.
func (c *Cliente) pedir(metodo, ruta string, cuerpo any, claveIdempotencia string) ([]byte, int, error) {
	var datos []byte
	if cuerpo != nil {
		datos, _ = json.Marshal(cuerpo)
	}

	for intento := 0; intento < 2; intento++ {
		token, err := c.obtenerToken(intento > 0)
		if err != nil {
			return nil, 0, err
		}
		pedido, err := http.NewRequest(metodo, c.BaseURL+ruta, bytes.NewReader(datos))
		if err != nil {
			return nil, 0, err
		}
		pedido.Header.Set("Authorization", "Bearer "+token)
		pedido.Header.Set("Content-Type", "application/json")
		if claveIdempotencia != "" {
			pedido.Header.Set("Idempotency-Key", claveIdempotencia)
		}

		respuesta, err := c.HTTP.Do(pedido)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", puertos.ErrNoDisponible, err)
		}
		var leido bytes.Buffer
		leido.ReadFrom(respuesta.Body)
		respuesta.Body.Close()

		switch respuesta.StatusCode {
		case http.StatusUnauthorized:
			continue
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			// Lo que no entra ahora sale en la vuelta siguiente de los procesos de fondo.
			return nil, respuesta.StatusCode, fmt.Errorf("%w: Rumbo respondió %d (Retry-After %s)",
				puertos.ErrNoDisponible, respuesta.StatusCode, respuesta.Header.Get("Retry-After"))
		}
		return leido.Bytes(), respuesta.StatusCode, nil
	}
	return nil, http.StatusUnauthorized, fmt.Errorf("%w: Rumbo rechazó el token nuevo", puertos.ErrNoDisponible)
}
