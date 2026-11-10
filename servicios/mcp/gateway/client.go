// Package gateway hace las llamadas a la API del sistema, con el token del usuario que está chateando.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Timeout es lo que esperamos a cada llamada: un poco más que lo que el gateway les da a los servicios
// (5 segundos para crear una transferencia, 2 para el resto), para recibir su respuesta en lugar de cortar antes.
const Timeout = 6 * time.Second

// Client llama al gateway. No tiene permisos propios: usa siempre el token que recibe.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// Do hace el pedido y devuelve el código y el cuerpo de la respuesta.
func (c Client) Do(ctx context.Context, method, path, authorization string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Authorization", authorization)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		// El error de net/http trae la URL, pero nunca los headers, así que el token no queda en los logs.
		return 0, nil, fmt.Errorf("el gateway no respondió: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("el gateway cortó la respuesta: %w", err)
	}
	return response.StatusCode, data, nil
}
