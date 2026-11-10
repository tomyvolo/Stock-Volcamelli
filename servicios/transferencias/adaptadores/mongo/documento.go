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
	Observaciones        string           `bson:"observaciones"`
	Estado               string           `bson:"estado"`
	Historial            []cambioDeEstado `bson:"historial"`
	IDRumbo              string           `bson:"id_rumbo"`
	Franja               *franja          `bson:"franja"`
	DemoraMinutos        int              `bson:"demora_minutos"`
	Motivo               string           `bson:"motivo"`
	ActualizadoEnRumbo   *time.Time       `bson:"actualizado_en_rumbo"`
	CancelacionPorEnviar *string          `bson:"cancelacion_por_enviar"`
	LiberacionPorEnviar  bool             `bson:"liberacion_por_enviar"`
}

type franja struct {
	Inicio time.Time `bson:"inicio"`
	Fin    time.Time `bson:"fin"`
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
	var f *franja
	if t.Franja != nil {
		f = &franja{Inicio: t.Franja.Inicio, Fin: t.Franja.Fin}
	}
	return documento{
		ID: t.ID, ProductoID: t.ProductoID, NombreProducto: t.NombreProducto, Cantidad: t.Cantidad,
		OrigenID: t.OrigenID, DestinoID: t.DestinoID,
		Origen:    ubicacion{Nombre: t.Origen.Nombre, Direccion: t.Origen.Direccion},
		Destino:   ubicacion{Nombre: t.Destino.Nombre, Direccion: t.Destino.Direccion},
		CreadaPor: t.CreadaPor, CreadaEn: t.CreadaEn, TrackID: t.TrackID,
		HoraLlegadaPedida: t.HoraLlegadaPedida, Observaciones: t.Observaciones,
		Estado: string(t.Estado), Historial: historial,
		IDRumbo: t.IDRumbo, Franja: f, DemoraMinutos: t.DemoraMinutos, Motivo: t.Motivo,
		ActualizadoEnRumbo: t.ActualizadoEnRumbo, CancelacionPorEnviar: t.CancelacionPorEnviar,
		LiberacionPorEnviar: t.LiberacionPorEnviar,
	}
}

func (d documento) aDominio() *dominio.Transferencia {
	historial := make([]dominio.CambioDeEstado, 0, len(d.Historial))
	for _, cambio := range d.Historial {
		historial = append(historial, dominio.CambioDeEstado{Estado: dominio.Estado(cambio.Estado), Fecha: cambio.Fecha, Detalle: cambio.Detalle})
	}
	var f *dominio.Franja
	if d.Franja != nil {
		f = &dominio.Franja{Inicio: d.Franja.Inicio, Fin: d.Franja.Fin}
	}
	return &dominio.Transferencia{
		ID: d.ID, ProductoID: d.ProductoID, NombreProducto: d.NombreProducto, Cantidad: d.Cantidad,
		OrigenID: d.OrigenID, DestinoID: d.DestinoID,
		Origen:    dominio.Ubicacion{Nombre: d.Origen.Nombre, Direccion: d.Origen.Direccion},
		Destino:   dominio.Ubicacion{Nombre: d.Destino.Nombre, Direccion: d.Destino.Direccion},
		CreadaPor: d.CreadaPor, CreadaEn: d.CreadaEn, TrackID: d.TrackID,
		HoraLlegadaPedida: d.HoraLlegadaPedida, Observaciones: d.Observaciones,
		Estado: dominio.Estado(d.Estado), Historial: historial,
		IDRumbo: d.IDRumbo, Franja: f, DemoraMinutos: d.DemoraMinutos, Motivo: d.Motivo,
		ActualizadoEnRumbo: d.ActualizadoEnRumbo, CancelacionPorEnviar: d.CancelacionPorEnviar,
		LiberacionPorEnviar: d.LiberacionPorEnviar,
	}
}
