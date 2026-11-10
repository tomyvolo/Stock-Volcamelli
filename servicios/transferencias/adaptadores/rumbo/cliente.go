// Package rumbo es el adaptador del Transporte para Rumbo, el servicio de traslados de otro grupo.
// Es el único lugar que conoce su contrato (docs/contracts/rumbo/traslados.yaml): si Rumbo
// cambia, solo se toca este paquete.
package rumbo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"transferencias/puertos"
)

// Timeout es lo que esperamos cada respuesta de Rumbo (ADR-010).
const Timeout = 5 * time.Second

// Circuit breaker (ADR-010): después de fallasParaAbrir fallas seguidas no llamamos a Rumbo durante tiempoAbierto.
const (
	fallasParaAbrir = 5
	tiempoAbierto   = 30 * time.Second
)

var (
	erroresRumbo = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rumbo_errors_total",
		Help: "Llamadas a Rumbo que fallaron por timeout, error de red o 5xx.",
	})
	aperturasCircuito = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rumbo_circuit_opens_total",
		Help: "Veces que se abrió el circuit breaker de Rumbo.",
	})
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

	// Estado del circuit breaker y del límite de pedidos.
	muCircuito   sync.Mutex
	fallas       int
	abiertoHasta time.Time
	esperarHasta time.Time // Rumbo respondió 429 y pidió esperar
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

// permitir dice si podemos llamar a Rumbo. Con el circuito abierto no lo llamamos. Pasados los 30 segundos
// queda medio abierto: solo dejamos pasar una llamada de prueba, que siempre es la consulta de cambios.
func (c *Cliente) permitir(esPrueba bool) error {
	c.muCircuito.Lock()
	defer c.muCircuito.Unlock()
	ahora := time.Now()
	if ahora.Before(c.esperarHasta) {
		return fmt.Errorf("%w: Rumbo pidió esperar hasta las %s", puertos.ErrNoDisponible, c.esperarHasta.Format("15:04:05"))
	}
	if c.fallas < fallasParaAbrir {
		return nil
	}
	if ahora.Before(c.abiertoHasta) || !esPrueba {
		return fmt.Errorf("%w: el circuit breaker de Rumbo está abierto", puertos.ErrNoDisponible)
	}
	// Medio abierto: mientras esta prueba no responde, las demás llamadas siguen sin pasar.
	c.abiertoHasta = ahora.Add(tiempoAbierto)
	return nil
}

// registrar anota cómo terminó una llamada. Una falla es un timeout, un error de red o un 5xx; los errores
// del pedido (4xx) no cuentan, porque Rumbo está andando.
func (c *Cliente) registrar(falla bool) {
	c.muCircuito.Lock()
	defer c.muCircuito.Unlock()
	if !falla {
		c.fallas = 0
		return
	}
	erroresRumbo.Inc()
	c.fallas++
	if c.fallas >= fallasParaAbrir {
		c.abiertoHasta = time.Now().Add(tiempoAbierto)
		aperturasCircuito.Inc()
		c.Registro.Escribir("", "warn", fmt.Sprintf("Rumbo falló %d veces seguidas: no lo llamamos por %v", c.fallas, tiempoAbierto))
	}
}

// esperar respeta el Retry-After de un 429. Si no viene, esperamos lo mismo que con el circuito abierto.
func (c *Cliente) esperar(retryAfter string) {
	segundos, err := strconv.Atoi(retryAfter)
	espera := time.Duration(segundos) * time.Second
	if err != nil || segundos <= 0 {
		espera = tiempoAbierto
	}
	c.muCircuito.Lock()
	c.esperarHasta = time.Now().Add(espera)
	c.muCircuito.Unlock()
}

// obtenerToken devuelve el token guardado o pide uno nuevo si falta poco para que venza.
func (c *Cliente) obtenerToken(renovar bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !renovar && c.token != "" && time.Until(c.tokenVence) > time.Minute {
		return c.token, nil
	}

	cuerpo, _ := json.Marshal(map[string]string{"client_id": c.ClientID, "client_secret": c.ClientSecret})
	respuesta, err := c.HTTP.Post(c.BaseURL+"/auth/token", "application/json", bytes.NewReader(cuerpo))
	if err != nil {
		c.registrar(true)
		return "", fmt.Errorf("%w: token: %v", puertos.ErrNoDisponible, err)
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		c.registrar(respuesta.StatusCode >= 500)
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
// Devuelve el cuerpo y el código. Los errores de red, los 429 y los 5xx se devuelven como ErrNoDisponible,
// para que el proceso de fondo reintente en la vuelta siguiente. esPrueba marca la consulta de cambios,
// la única que pasa con el circuito medio abierto.
func (c *Cliente) pedir(metodo, ruta string, cuerpo any, claveIdempotencia string, esPrueba bool) ([]byte, int, error) {
	if err := c.permitir(esPrueba); err != nil {
		return nil, 0, err
	}
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
			c.registrar(true)
			return nil, 0, fmt.Errorf("%w: %v", puertos.ErrNoDisponible, err)
		}
		var leido bytes.Buffer
		leido.ReadFrom(respuesta.Body)
		respuesta.Body.Close()

		switch {
		case respuesta.StatusCode == http.StatusUnauthorized:
			continue
		case respuesta.StatusCode == http.StatusTooManyRequests:
			c.esperar(respuesta.Header.Get("Retry-After"))
			return nil, respuesta.StatusCode, fmt.Errorf("%w: Rumbo respondió 429 (Retry-After %s)",
				puertos.ErrNoDisponible, respuesta.Header.Get("Retry-After"))
		case respuesta.StatusCode >= 500:
			// 502, 503 y 504: Rumbo no pudo procesarlo o está despertando. Se reintenta con la misma clave.
			c.registrar(true)
			return nil, respuesta.StatusCode, fmt.Errorf("%w: Rumbo respondió %d", puertos.ErrNoDisponible, respuesta.StatusCode)
		}
		c.registrar(false)
		return leido.Bytes(), respuesta.StatusCode, nil
	}
	return nil, http.StatusUnauthorized, fmt.Errorf("%w: Rumbo rechazó el token nuevo", puertos.ErrNoDisponible)
}
