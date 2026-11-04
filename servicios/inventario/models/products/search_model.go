package products

// SearchQuery es el pedido de búsqueda. No usa los nombres q, fq o rows:
// esos son detalles de Solr que se traducen recién en el repositorio.
type SearchQuery struct {
	Text     string
	Supplier string
	Page     int
	Size     int
}

// SearchResult es un producto encontrado, con su relevancia.
type SearchResult struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Supplier *string `json:"supplier"`
	Score    float64 `json:"score"`
}

// SearchResultList es una página de resultados y la cantidad total que encontró la búsqueda.
type SearchResultList struct {
	Results []SearchResult
	Total   int64
}
