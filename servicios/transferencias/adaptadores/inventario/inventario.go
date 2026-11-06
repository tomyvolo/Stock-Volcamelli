// Package inventario habla con Inventario por HTTP, a través de su NGINX y sin pasar por el gateway.
package inventario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Inventario implementa puertos.Inventario. El Client tiene que tener el timeout de 2 segundos.
type Inventario struct {
	BaseURL string
	Client  *http.Client
}

type errorInventario struct {
	Code string `json:"code"`
}

func (i Inventario) NombreProducto(productoID uint, trackID string) (string, error) {
	respuesta, err := i.pedir(http.MethodGet, fmt.Sprintf("/api/products/%d", productoID), nil, trackID)
	if err != nil {
		return "", err
	}
	defer respuesta.Body.Close()

	switch respuesta.StatusCode {
	case http.StatusOK:
		var producto struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(respuesta.Body).Decode(&producto); err != nil {
			return "", fmt.Errorf("%w: respuesta ilegible de Inventario: %v", puertos.ErrNoDisponible, err)
		}
		return producto.Name, nil
	case http.StatusNotFound:
		return "", puertos.ErrNoExiste
	default:
		return "", fmt.Errorf("%w: Inventario respondió %d", puertos.ErrNoDisponible, respuesta.StatusCode)
	}
}

// Reservar no reintenta: si no hay respuesta, no sabemos si reservó, y eso lo resuelve el centro.
func (i Inventario) Reservar(t *dominio.Transferencia, trackID string) error {
	cuerpo, _ := json.Marshal(map[string]any{
		"transferId": t.ID, "productId": t.ProductoID, "originBranchId": t.OrigenID,
		"destinationBranchId": t.DestinoID, "quantity": t.Cantidad,
	})
	respuesta, err := i.pedir(http.MethodPost, "/internal/reservations", cuerpo, trackID)
	if err != nil {
		return err
	}
	defer respuesta.Body.Close()

	var detalle errorInventario
	json.NewDecoder(respuesta.Body).Decode(&detalle)
	switch {
	case respuesta.StatusCode == http.StatusOK:
		return nil
	case respuesta.StatusCode == http.StatusConflict && detalle.Code == "INSUFFICIENT_STOCK":
		return puertos.ErrSinStock
	case respuesta.StatusCode == http.StatusNotFound:
		return puertos.ErrNoExiste
	default:
		// Incluye RESERVATION_RELEASED: la reserva se había liberado antes, así que esta transferencia no tiene nada reservado.
		return fmt.Errorf("%w: Inventario respondió %d %s", puertos.ErrNoDisponible, respuesta.StatusCode, detalle.Code)
	}
}

// Liberar se puede repetir sin problema: Inventario no libera dos veces la misma reserva.
func (i Inventario) Liberar(transferenciaID, trackID string) error {
	respuesta, err := i.pedir(http.MethodPost, "/internal/reservations/"+transferenciaID+"/release", nil, trackID)
	if err != nil {
		return err
	}
	defer respuesta.Body.Close()
	// 409 es que la reserva ya estaba confirmada: no hay nada que liberar.
	if respuesta.StatusCode == http.StatusOK || respuesta.StatusCode == http.StatusConflict {
		return nil
	}
	return fmt.Errorf("%w: Inventario respondió %d", puertos.ErrNoDisponible, respuesta.StatusCode)
}

// pedir manda el pedido con el TrackID, así queda en la misma traza.
// NGINX responde 502 o 504 si ninguna instancia de Inventario contestó.
func (i Inventario) pedir(metodo, ruta string, cuerpo []byte, trackID string) (*http.Response, error) {
	pedido, err := http.NewRequest(metodo, i.BaseURL+ruta, bytes.NewReader(cuerpo))
	if err != nil {
		return nil, err
	}
	pedido.Header.Set("Content-Type", "application/json")
	pedido.Header.Set("X-Track-ID", trackID)
	respuesta, err := i.Client.Do(pedido)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", puertos.ErrNoDisponible, err)
	}
	if respuesta.StatusCode == http.StatusBadGateway || respuesta.StatusCode == http.StatusGatewayTimeout {
		respuesta.Body.Close()
		return nil, fmt.Errorf("%w: NGINX respondió %d", puertos.ErrNoDisponible, respuesta.StatusCode)
	}
	return respuesta, nil
}
