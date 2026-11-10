package products

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"inventario/models/products"
)

// fakeSolr responde como Solr y guarda lo que le pidieron.
type fakeSolr struct {
	existing map[string]bool // campos que ya tiene el esquema
	queries  []string
	schema   string
}

func (f *fakeSolr) handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/schema/fields/"):
		if !f.existing[strings.TrimPrefix(r.URL.Path, "/schema/fields/")] {
			w.WriteHeader(http.StatusNotFound)
		}
	case r.URL.Path == "/schema":
		body, _ := io.ReadAll(r.Body)
		f.schema = string(body)
	case r.URL.Path == "/select":
		f.queries = append(f.queries, r.URL.RawQuery)
		io.WriteString(w, `{"response":{"numFound":0,"docs":[]}}`)
	}
}

func newFakeSolr(existing ...string) (*fakeSolr, SearchSolr, func()) {
	fake := &fakeSolr{existing: map[string]bool{}}
	for _, name := range existing {
		fake.existing[name] = true
	}
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	return fake, SearchSolr{BaseURL: server.URL, Client: server.Client()}, server.Close
}

// Por nombre se ordena con nombre_orden; por relevancia no se manda sort y Solr usa el score.
func TestSearchSolr_Sort(t *testing.T) {
	fake, solr, closeServer := newFakeSolr()
	defer closeServer()

	solr.Search(products.SearchQuery{Text: "harina", Page: 1, Size: 20, Sort: products.SortName})
	solr.Search(products.SearchQuery{Text: "harina", Page: 1, Size: 20, Sort: products.SortRelevance})

	if !strings.Contains(fake.queries[0], "sort=nombre_orden+asc") {
		t.Errorf("Por nombre se esperaba sort=nombre_orden asc, pero se pidió: %s", fake.queries[0])
	}
	if strings.Contains(fake.queries[1], "sort=") {
		t.Errorf("Por relevancia no se esperaba sort, pero se pidió: %s", fake.queries[1])
	}
}

// Un índice que ya existía antes de sumar nombre_orden recibe solo ese campo.
func TestCreateFields_SoloLosQueFaltan(t *testing.T) {
	fake, solr, closeServer := newFakeSolr("nombre", "descripcion", "proveedor")
	defer closeServer()

	if err := solr.CreateFields(); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if !strings.Contains(fake.schema, "nombre_orden") || strings.Contains(fake.schema, "descripcion") {
		t.Errorf("Se esperaba que cree solo nombre_orden, pero mandó: %s", fake.schema)
	}
}
