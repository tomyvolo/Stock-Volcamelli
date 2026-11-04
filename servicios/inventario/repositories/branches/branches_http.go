package branches

import (
	"fmt"
	"net/http"
)

// BranchesHTTP le pregunta a Sucursales por HTTP, directo y sin pasar por el gateway.
// El Client tiene que tener el timeout de 2 segundos de las llamadas internas.
type BranchesHTTP struct {
	BaseURL string
	Client  *http.Client
}

func (repo BranchesHTTP) Exists(branchID uint, trackID string) (bool, error) {
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/branches/%d", repo.BaseURL, branchID), nil)
	if err != nil {
		return false, err
	}
	// Así el pedido a Sucursales queda en la misma traza.
	request.Header.Set("X-Track-ID", trackID)

	response, err := repo.Client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("sucursales respondió %d", response.StatusCode)
	}
}
