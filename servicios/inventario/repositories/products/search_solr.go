package products

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"inventario/models/products"
)

// SearchSolr es el único lugar de Inventario que conoce la URL, los parámetros y las respuestas de Solr.
// BaseURL apunta al índice, por ejemplo http://solr:8983/solr/productos.
type SearchSolr struct {
	BaseURL string
	Client  *http.Client
}

// solrDocument es un producto como lo guardamos en Solr. Los campos están en ADR-006.
type solrDocument struct {
	ID          string  `json:"id"`
	Nombre      string  `json:"nombre"`
	Descripcion string  `json:"descripcion,omitempty"`
	Proveedor   *string `json:"proveedor,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

// solrResponse es solo la parte de la respuesta de Solr que usamos.
type solrResponse struct {
	Response struct {
		NumFound int64          `json:"numFound"`
		Docs     []solrDocument `json:"docs"`
	} `json:"response"`
}

// fields son los campos del índice. id ya viene en el esquema de Solr.
var fields = []map[string]any{
	{"name": "nombre", "type": "text_es", "indexed": true, "stored": true, "multiValued": false},
	{"name": "descripcion", "type": "text_es", "indexed": true, "stored": true, "multiValued": false},
	{"name": "proveedor", "type": "string", "indexed": true, "stored": true, "multiValued": false},
}

func (repo SearchSolr) Search(query products.SearchQuery) (products.SearchResultList, error) {
	// Una coincidencia en el nombre pesa 5 veces más que una en la descripción.
	params := url.Values{
		"q":       {query.Text},
		"defType": {"edismax"},
		"qf":      {"nombre^5 descripcion"},
		"start":   {strconv.Itoa((query.Page - 1) * query.Size)},
		"rows":    {strconv.Itoa(query.Size)},
		"fl":      {"id,nombre,proveedor,score"},
		"wt":      {"json"},
	}
	// {!term} toma el proveedor como valor literal, no como sintaxis de Solr, y no cambia la relevancia.
	if query.Supplier != "" {
		params.Add("fq", "{!term f=proveedor}"+query.Supplier)
	}

	response, err := repo.Client.Get(repo.BaseURL + "/select?" + params.Encode())
	if err != nil {
		return products.SearchResultList{}, fmt.Errorf("consultar Solr: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return products.SearchResultList{}, fmt.Errorf("Solr respondió HTTP %d", response.StatusCode)
	}

	var payload solrResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return products.SearchResultList{}, fmt.Errorf("leer la respuesta de Solr: %w", err)
	}

	results := make([]products.SearchResult, 0, len(payload.Response.Docs))
	for _, document := range payload.Response.Docs {
		id, err := strconv.ParseUint(document.ID, 10, 32)
		if err != nil {
			return products.SearchResultList{}, fmt.Errorf("Solr devolvió un id que no es un número: %q", document.ID)
		}
		results = append(results, products.SearchResult{
			ID: uint(id), Name: document.Nombre, Supplier: document.Proveedor, Score: document.Score,
		})
	}
	return products.SearchResultList{Results: results, Total: payload.Response.NumFound}, nil
}

// SaveProducts manda los documentos enteros: si ya existía uno con ese id, Solr lo reemplaza.
// commitWithin hace que el cambio aparezca en la búsqueda en menos de 1 segundo.
func (repo SearchSolr) SaveProducts(list []products.ProductModel) error {
	documents := make([]solrDocument, 0, len(list))
	for _, product := range list {
		documents = append(documents, solrDocument{
			ID: strconv.FormatUint(uint64(product.ID), 10), Nombre: product.Name,
			Descripcion: product.Description, Proveedor: product.Supplier,
		})
	}
	return repo.post("/update?commitWithin=1000", documents)
}

// CreateFields pregunta si ya existe el campo nombre. Si Solr responde 404, el índice es nuevo y crea los campos.
func (repo SearchSolr) CreateFields() error {
	response, err := repo.Client.Get(repo.BaseURL + "/schema/fields/nombre")
	if err != nil {
		return fmt.Errorf("consultar el esquema de Solr: %w", err)
	}
	response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return nil
	}
	if response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("Solr respondió HTTP %d al consultar el esquema", response.StatusCode)
	}
	return repo.post("/schema", map[string]any{"add-field": fields})
}

func (repo SearchSolr) post(path string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	response, err := repo.Client.Post(repo.BaseURL+path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("escribir en Solr: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Solr respondió HTTP %d", response.StatusCode)
	}
	return nil
}
