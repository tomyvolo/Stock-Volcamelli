// Package mongo guarda las transferencias en MongoDB, una por documento, con su historial adentro.
package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"transferencias/dominio"
	"transferencias/puertos"
)

// Como las llamadas entre servicios, ninguna operación con MongoDB espera más de 2 segundos.
const timeout = 2 * time.Second

// Repositorio implementa puertos.Repositorio.
type Repositorio struct {
	transferencias *mongo.Collection
	procesos       *mongo.Collection
}

// Conectar abre la conexión y crea los índices de las consultas que hacemos: por estado (los procesos
// de fondo) y por sucursal y fecha (las pantallas).
func Conectar(uri, base string) (*Repositorio, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := client.Database(base)
	repo := &Repositorio{transferencias: db.Collection("transferencias"), procesos: db.Collection("procesos")}
	_, err = repo.transferencias.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "estado", Value: 1}}},
		{Keys: bson.D{{Key: "origen_id", Value: 1}, {Key: "creada_en", Value: -1}}},
		{Keys: bson.D{{Key: "destino_id", Value: 1}, {Key: "creada_en", Value: -1}}},
	})
	return repo, err
}

// Guardar reemplaza el documento entero. En MongoDB escribir un documento es atómico, así que
// el estado y su historial se guardan juntos.
func (r *Repositorio) Guardar(t *dominio.Transferencia) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := r.transferencias.ReplaceOne(ctx, bson.M{"_id": t.ID}, aDocumento(t), options.Replace().SetUpsert(true))
	return err
}

func (r *Repositorio) Obtener(id string) (*dominio.Transferencia, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var doc documento
	err := r.transferencias.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, puertos.ErrNoExiste
	}
	if err != nil {
		return nil, err
	}
	return doc.aDominio(), nil
}

// Listar devuelve las más nuevas primero.
func (r *Repositorio) Listar(filtro puertos.Filtro) ([]*dominio.Transferencia, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	consulta := bson.M{}
	if filtro.SucursalID != 0 {
		consulta["$or"] = bson.A{bson.M{"origen_id": filtro.SucursalID}, bson.M{"destino_id": filtro.SucursalID}}
	}
	if filtro.Estado != "" {
		consulta["estado"] = filtro.Estado
	}
	total, err := r.transferencias.CountDocuments(ctx, consulta)
	if err != nil {
		return nil, 0, err
	}

	opciones := options.Find().SetSort(bson.D{{Key: "creada_en", Value: -1}}).
		SetSkip(int64((filtro.Pagina - 1) * filtro.Tamanio)).SetLimit(int64(filtro.Tamanio))
	return r.buscar(ctx, consulta, opciones, total)
}

// PendientesDeEnvio busca lo que los procesos de fondo tienen que mandar.
func (r *Repositorio) PendientesDeEnvio() ([]*dominio.Transferencia, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	consulta := bson.M{"$or": bson.A{
		bson.M{"estado": dominio.Reservada, "id_rumbo": ""},
		bson.M{"cancelacion_por_enviar": bson.M{"$ne": nil}},
		bson.M{"liberacion_por_enviar": true},
	}}
	lista, _, err := r.buscar(ctx, consulta, options.Find().SetSort(bson.D{{Key: "creada_en", Value: 1}}), 0)
	return lista, err
}

func (r *Repositorio) buscar(ctx context.Context, consulta bson.M, opciones *options.FindOptions, total int64) ([]*dominio.Transferencia, int64, error) {
	cursor, err := r.transferencias.Find(ctx, consulta, opciones)
	if err != nil {
		return nil, 0, err
	}
	var docs []documento
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	lista := make([]*dominio.Transferencia, 0, len(docs))
	for _, doc := range docs {
		lista = append(lista, doc.aDominio())
	}
	return lista, total, nil
}

// UltimaConsulta devuelve la fecha cero si todavía no consultamos nunca: Rumbo devuelve todo.
func (r *Repositorio) UltimaConsulta() (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var doc struct {
		Hasta time.Time `bson:"hasta"`
	}
	err := r.procesos.FindOne(ctx, bson.M{"_id": "consulta_rumbo"}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, nil
	}
	return doc.Hasta, err
}

func (r *Repositorio) GuardarUltimaConsulta(hasta time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := r.procesos.ReplaceOne(ctx, bson.M{"_id": "consulta_rumbo"}, bson.M{"hasta": hasta}, options.Replace().SetUpsert(true))
	return err
}
