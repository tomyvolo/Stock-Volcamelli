package rumbo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Rumbo opera en hora de Argentina; le mandamos las fechas y horas con esa zona.
var argentina = time.FixedZone("ART", -3*60*60)

type ubicacion struct {
	Nombre    string `json:"nombre"`
	Direccion string `json:"direccion"`
}

type cosa struct {
	Descripcion string `json:"descripcion"`
	Cantidad    int    `json:"cantidad"`
}

type nuevoTraslado struct {
	ReferenciaExterna string    `json:"referencia_externa"`
	Cosa              cosa      `json:"cosa"`
	Origen            ubicacion `json:"origen"`
	Destino           ubicacion `json:"destino"`
	FranjaInicio      string    `json:"franja_inicio"`
	Observaciones     string    `json:"observaciones,omitempty"`
}

// Franjas consulta la disponibilidad de un día y devuelve las franjas en las que todavía entra un traslado.
func (c *Cliente) Franjas(fecha time.Time) ([]dominio.Franja, error) {
	cuerpo, codigo, err := c.pedir(http.MethodGet, "/disponibilidad?fecha="+fecha.In(argentina).Format("2006-01-02"), nil, "", false)
	if err != nil {
		return nil, err
	}
	switch codigo {
	case http.StatusOK:
	case http.StatusUnprocessableEntity:
		// El día está fuera de lo que se puede reservar: no tiene franjas.
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: Rumbo respondió %d a la disponibilidad", puertos.ErrNoDisponible, codigo)
	}

	var disponibilidad struct {
		Franjas []struct {
			Inicio     time.Time `json:"inicio"`
			Fin        time.Time `json:"fin"`
			Disponible bool      `json:"disponible"`
		} `json:"franjas"`
	}
	if err := json.Unmarshal(cuerpo, &disponibilidad); err != nil {
		return nil, fmt.Errorf("%w: disponibilidad ilegible: %v", puertos.ErrNoDisponible, err)
	}
	var franjas []dominio.Franja
	for _, f := range disponibilidad.Franjas {
		if f.Disponible {
			franjas = append(franjas, dominio.Franja{Inicio: f.Inicio, Fin: f.Fin})
		}
	}
	slices.SortFunc(franjas, func(a, b dominio.Franja) int { return a.Inicio.Compare(b.Inicio) })
	return franjas, nil
}

// Reservar reserva el traslado en la franja elegida. La Idempotency-Key es el id de la transferencia con
// el inicio de la franja: un reintento a la misma franja lleva la misma clave y Rumbo no reserva dos veces,
// y otra franja es otra intención, con otra clave, como pide su contrato.
func (c *Cliente) Reservar(t *dominio.Transferencia) (dominio.Novedad, error) {
	if t.Franja == nil {
		return dominio.Novedad{}, errors.New("la transferencia no tiene franja elegida")
	}
	pedido := nuevoTraslado{
		ReferenciaExterna: t.ID,
		Cosa:              cosa{Descripcion: t.NombreProducto, Cantidad: t.Cantidad},
		Origen:            ubicacion{Nombre: t.Origen.Nombre, Direccion: t.Origen.Direccion},
		Destino:           ubicacion{Nombre: t.Destino.Nombre, Direccion: t.Destino.Direccion},
		FranjaInicio:      t.Franja.Inicio.In(argentina).Format(time.RFC3339),
		Observaciones:     t.Observaciones,
	}
	clave := fmt.Sprintf("%s-%d", t.ID, t.Franja.Inicio.Unix())

	cuerpo, codigo, err := c.pedir(http.MethodPost, "/traslados", pedido, clave, false)
	if err != nil {
		return dominio.Novedad{}, err
	}
	if codigo == http.StatusCreated {
		return leerTraslado(cuerpo)
	}

	var p problema
	json.Unmarshal(cuerpo, &p)
	switch {
	case codigo == http.StatusConflict && p.Codigo == "SIN_DISPONIBILIDAD",
		codigo == http.StatusUnprocessableEntity && (p.Codigo == "ANTICIPACION_INSUFICIENTE" || p.Codigo == "HORARIO_EN_EL_PASADO"):
		// Otro grupo tomó el último lugar, o reintentamos tarde y la franja ya no se puede reservar.
		// En los dos casos Rumbo no reservó nada.
		return dominio.Novedad{}, fmt.Errorf("%w: %s", puertos.ErrSinLugar, p.Codigo)
	case codigo >= 400 && codigo < 500:
		// Datos inválidos, excede la capacidad, la clave ya se usó con otro pedido o un código que no
		// conocemos: es un problema del pedido y reintentar no lo arregla.
		return dominio.Novedad{Motivo: "Rumbo rechazó el pedido: " + p.motivo()},
			fmt.Errorf("%w: Rumbo respondió %d %s", puertos.ErrRechazoDelPedido, codigo, p.Codigo)
	default:
		return dominio.Novedad{}, fmt.Errorf("%w: Rumbo respondió %d al reservar", puertos.ErrNoDisponible, codigo)
	}
}

// Cambios trae todas las páginas de traslados que cambiaron después de desde, en orden.
func (c *Cliente) Cambios(desde time.Time) ([]dominio.Novedad, time.Time, error) {
	hasta := desde
	var novedades []dominio.Novedad

	for pagina := 1; ; pagina++ {
		parametros := url.Values{"pagina": {fmt.Sprint(pagina)}, "tamanio": {"100"}}
		if !desde.IsZero() {
			// Pedimos desde un segundo antes: si dos traslados cambiaron en el mismo segundo y consultamos
			// entre los dos, no perdemos el segundo. Lo que llega repetido no cambia nada.
			parametros.Set("actualizado_desde", desde.Add(-time.Second).In(argentina).Format(time.RFC3339))
		}
		cuerpo, codigo, err := c.pedir(http.MethodGet, "/traslados?"+parametros.Encode(), nil, "", true)
		if err != nil {
			return nil, desde, err
		}
		if codigo != http.StatusOK {
			return nil, desde, fmt.Errorf("%w: Rumbo respondió %d al listar los cambios", puertos.ErrNoDisponible, codigo)
		}

		var resultado struct {
			Resultados []traslado `json:"resultados"`
			Total      int        `json:"total"`
		}
		if err := json.Unmarshal(cuerpo, &resultado); err != nil {
			return nil, desde, fmt.Errorf("%w: lista ilegible: %v", puertos.ErrNoDisponible, err)
		}
		for _, tr := range resultado.Resultados {
			if tr.ActualizadoEn.After(hasta) {
				hasta = tr.ActualizadoEn
			}
			n, err := traducir(tr)
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

// Consultar trae el estado actual de un traslado.
func (c *Cliente) Consultar(t *dominio.Transferencia) (dominio.Novedad, error) {
	cuerpo, codigo, err := c.pedir(http.MethodGet, "/traslados/"+url.PathEscape(t.IDRumbo), nil, "", false)
	if err != nil {
		return dominio.Novedad{}, err
	}
	if codigo != http.StatusOK {
		return dominio.Novedad{}, leerProblema(cuerpo, codigo)
	}
	return leerTraslado(cuerpo)
}

// Cancelar le pide a Rumbo que cancele. Rumbo la aplica en el momento; con el traslado en curso responde 409.
func (c *Cliente) Cancelar(t *dominio.Transferencia, motivo string) (dominio.Novedad, error) {
	cuerpo, codigo, err := c.pedir(http.MethodPost, "/traslados/"+url.PathEscape(t.IDRumbo)+"/cancelacion",
		map[string]string{"motivo": motivo}, t.ID+"-cancelacion", false)
	if err != nil {
		return dominio.Novedad{}, err
	}
	if codigo != http.StatusOK {
		return dominio.Novedad{}, leerProblema(cuerpo, codigo)
	}
	return leerTraslado(cuerpo)
}

func leerTraslado(cuerpo []byte) (dominio.Novedad, error) {
	var tr traslado
	if err := json.Unmarshal(cuerpo, &tr); err != nil {
		return dominio.Novedad{}, fmt.Errorf("%w: traslado ilegible: %v", puertos.ErrNoDisponible, err)
	}
	return traducir(tr)
}

// leerProblema traduce un error de Rumbo (tabla de errores de ADR-009).
func leerProblema(cuerpo []byte, codigo int) error {
	var p problema
	json.Unmarshal(cuerpo, &p)
	switch {
	case codigo == http.StatusConflict && p.Codigo == "ESTADO_NO_PERMITE_OPERACION":
		return fmt.Errorf("%w: %s", puertos.ErrEstadoNoPermite, p.motivo())
	case codigo >= 400 && codigo < 500:
		return fmt.Errorf("%w: Rumbo respondió %d %s", puertos.ErrRechazoDelPedido, codigo, p.Codigo)
	default:
		return fmt.Errorf("%w: Rumbo respondió %d %s", puertos.ErrNoDisponible, codigo, p.Codigo)
	}
}
