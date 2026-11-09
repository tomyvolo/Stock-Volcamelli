package mongo

import (
	"time"

	"transferencias/dominio"
)

// documento es cómo queda una transferencia en MongoDB. Lo traducimos acá para que el dominio
// no dependa de los tags de bson.
type documento struct {
	ID                   string           `bson:"_id"`
	ProductoID           uint             `bson:"producto_id"`
	NombreProducto       string           `bson:"nombre_producto"`
	Cantidad             int              `bson:"cantidad"`
	OrigenID             uint             `bson:"origen_id"`
	DestinoID            uint             `bson:"destino_id"`
	Origen               ubicacion        `bson:"origen"`
	Destino              ubicacion        `bson:"destino"`
	CreadaPor            uint             `bson:"creada_por"`
	CreadaEn             time.Time        `bson:"creada_en"`
	TrackID              string           `bson:"track_id"`
	HoraLlegadaPedida    *time.Time       `bson:"hora_llegada_pedida"`
	ToleranciaMinutos    int              `bson:"tolerancia_minutos"`
	Observaciones        string           `bson:"observaciones"`
	Estado               string           `bson:"estado"`
	Historial            []cambioDeEstado `bson:"historial"`
	IDRumbo              string           `bson:"id_rumbo"`
	HoraPropuesta        *time.Time       `bson:"hora_propuesta"`
	VencePropuesta       *time.Time       `bson:"vence_propuesta"`
	HoraComprometida     *time.Time       `bson:"hora_comprometida"`
	DemoraMinutos        int              `bson:"demora_minutos"`
	Motivo               string           `bson:"motivo"`
	ActualizadoEnRumbo   *time.Time       `bson:"actualizado_en_rumbo"`
	RespuestaPorEnviar   *bool            `bson:"respuesta_por_enviar"`
	CancelacionPorEnviar *string          `bson:"cancelacion_por_enviar"`
	LiberacionPorEnviar  bool             `bson:"liberacion_por_enviar"`
	CancelacionRechazada bool             `bson:"cancelacion_rechazada"`
}

type ubicacion struct {
	Nombre    string `bson:"nombre"`
	Direccion string `bson:"direccion"`
}

type cambioDeEstado struct {
	Estado  string    `bson:"estado"`
	Fecha   time.Time `bson:"fecha"`
	Detalle string    `bson:"detalle"`
}

func aDocumento(t *dominio.Transferencia) documento {
	historial := make([]cambioDeEstado, 0, len(t.Historial))
	for _, cambio := range t.Historial {
		historial = append(historial, cambioDeEstado{Estado: string(cambio.Estado), Fecha: cambio.Fecha, Detalle: cambio.Detalle})
	}
	return documento{
		ID: t.ID, ProductoID: t.ProductoID, NombreProducto: t.NombreProducto, Cantidad: t.Cantidad,
		OrigenID: t.OrigenID, DestinoID: t.DestinoID,
		Origen:    ubicacion{Nombre: t.Origen.Nombre, Direccion: t.Origen.Direccion},
		Destino:   ubicacion{Nombre: t.Destino.Nombre, Direccion: t.Destino.Direccion},
		CreadaPor: t.CreadaPor, CreadaEn: t.CreadaEn, TrackID: t.TrackID,
		HoraLlegadaPedida: t.HoraLlegadaPedida, ToleranciaMinutos: t.ToleranciaMinutos, Observaciones: t.Observaciones,
		Estado: string(t.Estado), Historial: historial,
		IDRumbo: t.IDRumbo, HoraPropuesta: t.HoraPropuesta, VencePropuesta: t.VencePropuesta,
		HoraComprometida: t.HoraComprometida, DemoraMinutos: t.DemoraMinutos, Motivo: t.Motivo,
		ActualizadoEnRumbo: t.ActualizadoEnRumbo, RespuestaPorEnviar: t.RespuestaPorEnviar,
		CancelacionPorEnviar: t.CancelacionPorEnviar, LiberacionPorEnviar: t.LiberacionPorEnviar,
		CancelacionRechazada: t.CancelacionRechazada,
	}
}

func (d documento) aDominio() *dominio.Transferencia {
	historial := make([]dominio.CambioDeEstado, 0, len(d.Historial))
	for _, cambio := range d.Historial {
		historial = append(historial, dominio.CambioDeEstado{Estado: dominio.Estado(cambio.Estado), Fecha: cambio.Fecha, Detalle: cambio.Detalle})
	}
	return &dominio.Transferencia{
		ID: d.ID, ProductoID: d.ProductoID, NombreProducto: d.NombreProducto, Cantidad: d.Cantidad,
		OrigenID: d.OrigenID, DestinoID: d.DestinoID,
		Origen:    dominio.Ubicacion{Nombre: d.Origen.Nombre, Direccion: d.Origen.Direccion},
		Destino:   dominio.Ubicacion{Nombre: d.Destino.Nombre, Direccion: d.Destino.Direccion},
		CreadaPor: d.CreadaPor, CreadaEn: d.CreadaEn, TrackID: d.TrackID,
		HoraLlegadaPedida: d.HoraLlegadaPedida, ToleranciaMinutos: d.ToleranciaMinutos, Observaciones: d.Observaciones,
		Estado: dominio.Estado(d.Estado), Historial: historial,
		IDRumbo: d.IDRumbo, HoraPropuesta: d.HoraPropuesta, VencePropuesta: d.VencePropuesta,
		HoraComprometida: d.HoraComprometida, DemoraMinutos: d.DemoraMinutos, Motivo: d.Motivo,
		ActualizadoEnRumbo: d.ActualizadoEnRumbo, RespuestaPorEnviar: d.RespuestaPorEnviar,
		CancelacionPorEnviar: d.CancelacionPorEnviar, LiberacionPorEnviar: d.LiberacionPorEnviar,
		CancelacionRechazada: d.CancelacionRechazada,
	}
}
