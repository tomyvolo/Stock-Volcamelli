package rumbo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Rumbo opera en hora de Argentina; le mandamos las horas con esa zona.
var argentina = time.FixedZone("ART", -3*60*60)

type ubicacion struct {
	Nombre    string `json:"nombre"`
	Direccion string `json:"direccion"`
}

type nuevoTraslado struct {
	ReferenciaExterna     string    `json:"referencia_externa"`
	Origen                ubicacion `json:"origen"`
	Destino               ubicacion `json:"destino"`
	HoraLlegadaSolicitada string    `json:"hora_llegada_solicitada,omitempty"`
	ToleranciaMinutos     int       `json:"tolerancia_minutos"`
	Cosa                  struct {
		Descripcion string `json:"descripcion"`
		Cantidad    int    `json:"cantidad"`
	} `json:"cosa"`
	Observaciones string `json:"observaciones,omitempty"`
}

// Solicitar pide el traslado. La Idempotency-Key es el id de la transferencia: si reintentamos,
// Rumbo no crea una solicitud repetida.
func (c *Cliente) Solicitar(t *dominio.Transferencia) (dominio.Novedad, error) {
	pedido := nuevoTraslado{
		ReferenciaExterna: t.ID,
		Origen:            ubicacion{Nombre: t.Origen.Nombre, Direccion: t.Origen.Direccion},
		Destino:           ubicacion{Nombre: t.Destino.Nombre, Direccion: t.Destino.Direccion},
		ToleranciaMinutos: t.ToleranciaMinutos,
		Observaciones:     t.Observaciones,
	}
	pedido.Cosa.Descripcion = t.NombreProducto
	pedido.Cosa.Cantidad = t.Cantidad
	if t.HoraLlegadaPedida != nil {
		pedido.HoraLlegadaSolicitada = t.HoraLlegadaPedida.In(argentina).Format(time.RFC3339)
	}

	cuerpo, codigo, err := c.pedir(http.MethodPost, "/v2/traslados", pedido, t.ID)
	if err != nil {
		return dominio.Novedad{}, err
	}
	return c.leerSolicitud(cuerpo, codigo, http.StatusAccepted)
}

// Cambios trae todas las páginas de solicitudes que cambiaron después de desde, en orden.
func (c *Cliente) Cambios(desde time.Time) ([]dominio.Novedad, time.Time, error) {
	hasta := desde
	var novedades []dominio.Novedad

	for pagina := 1; ; pagina++ {
		parametros := url.Values{"pagina": {fmt.Sprint(pagina)}, "tamanio": {"100"}}
		if !desde.IsZero() {
			// Pedimos desde un segundo antes: si dos solicitudes cambiaron en el mismo segundo y consultamos
			// entre las dos, no perdemos la segunda. Lo que llega repetido no cambia nada.
			parametros.Set("actualizado_desde", desde.Add(-time.Second).In(argentina).Format(time.RFC3339))
		}
		cuerpo, codigo, err := c.pedir(http.MethodGet, "/v2/traslados?"+parametros.Encode(), nil, "")
		if err != nil {
			return nil, desde, err
		}
		if codigo != http.StatusOK {
			return nil, desde, fmt.Errorf("%w: Rumbo respondió %d al listar los cambios", puertos.ErrNoDisponible, codigo)
		}

		var resultado struct {
			Resultados []solicitud `json:"resultados"`
			Total      int         `json:"total"`
		}
		if err := json.Unmarshal(cuerpo, &resultado); err != nil {
			return nil, desde, fmt.Errorf("%w: lista ilegible: %v", puertos.ErrNoDisponible, err)
		}
		for _, s := range resultado.Resultados {
			if s.ActualizadoEn.After(hasta) {
				hasta = s.ActualizadoEn
			}
			n, err := traducir(s)
			if err != nil {
				// Un estado que no conocemos no frena al resto: queda en el log y la transferencia no cambia.
				c.Registro.Escribir("", "warn", err.Error())
				continue
			}
			novedades = append(novedades, n)
		}
		if len(resultado.Resultados) == 0 || pagina*100 >= resultado.Total {
			return novedades, hasta, nil
		}
	}
}

// Consultar trae el estado actual de una solicitud.
func (c *Cliente) Consultar(t *dominio.Transferencia) (dominio.Novedad, error) {
	cuerpo, codigo, err := c.pedir(http.MethodGet, "/v2/traslados/"+t.IDRumbo, nil, "")
	if err != nil {
		return dominio.Novedad{}, err
	}
	return c.leerSolicitud(cuerpo, codigo, http.StatusOK)
}

// ResponderPropuesta acepta o rechaza el horario propuesto. Cada intención lleva su propia clave.
func (c *Cliente) ResponderPropuesta(t *dominio.Transferencia, acepta bool) (dominio.Novedad, error) {
	cuerpo, codigo, err := c.pedir(http.MethodPost, "/v2/traslados/"+t.IDRumbo+"/respuesta-contrapropuesta",
		map[string]bool{"acepta": acepta}, fmt.Sprintf("%s-respuesta-%t", t.ID, acepta))
	if err != nil {
		return dominio.Novedad{}, err
	}
	return c.leerSolicitud(cuerpo, codigo, http.StatusOK)
}

// Cancelar le pide a Rumbo que cancele. Con el traslado en curso, Rumbo responde 409.
func (c *Cliente) Cancelar(t *dominio.Transferencia, motivo string) (dominio.Novedad, error) {
	cuerpo, codigo, err := c.pedir(http.MethodPost, "/v2/traslados/"+t.IDRumbo+"/cancelacion",
		map[string]string{"motivo": motivo}, t.ID+"-cancelacion")
	if err != nil {
		return dominio.Novedad{}, err
	}
	return c.leerSolicitud(cuerpo, codigo, http.StatusOK)
}

// leerSolicitud traduce la respuesta según su código (tabla de errores de ADR-009).
func (c *Cliente) leerSolicitud(cuerpo []byte, codigo, esperado int) (dominio.Novedad, error) {
	if codigo == esperado {
		var s solicitud
		if err := json.Unmarshal(cuerpo, &s); err != nil {
			return dominio.Novedad{}, fmt.Errorf("%w: solicitud ilegible: %v", puertos.ErrNoDisponible, err)
		}
		return traducir(s)
	}

	var p problema
	json.Unmarshal(cuerpo, &p)
	switch {
	case codigo == http.StatusUnprocessableEntity:
		// Datos inválidos, hora en el pasado, poca anticipación o excede la capacidad: no se reintenta.
		return dominio.Novedad{Motivo: p.motivo()}, fmt.Errorf("%w: %s", puertos.ErrRechazoDelPedido, p.Codigo)
	case codigo == http.StatusConflict && p.Codigo == "CLAVE_IDEMPOTENCIA_REUTILIZADA":
		// Es un error nuestro: la misma clave con otro cuerpo. No se resuelve reintentando.
		return dominio.Novedad{}, fmt.Errorf("Rumbo rechazó la Idempotency-Key por reutilizada: %s", p.motivo())
	case codigo == http.StatusConflict:
		return dominio.Novedad{}, fmt.Errorf("%w: %s", puertos.ErrEstadoNoPermite, p.Codigo)
	default:
		return dominio.Novedad{}, fmt.Errorf("%w: Rumbo respondió %d %s", puertos.ErrNoDisponible, codigo, p.Codigo)
	}
}
