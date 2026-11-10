package products

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"inventario/models/products"
	repo "inventario/repositories/products"
)

var (
	ErrProductNotFound = errors.New("producto no encontrado")
	ErrNameRequired    = errors.New("el nombre del producto no puede estar vacío")
)

// Publisher avisa que se dio de alta o cambió un producto, para que se actualice la búsqueda.
// Lo implementa messaging con RabbitMQ.
type Publisher interface {
	ProductUpdated(productID uint, trackID string)
}

type ProductsService struct {
	Repo      repo.ProductsRepo
	Search    repo.SearchRepo
	Publisher Publisher
}

// rebuildPageSize es de a cuántos productos se leen de MySQL y se mandan a Solr al reconstruir el índice.
const rebuildPageSize = 100

func (s *ProductsService) GetProducts(page, size int) (products.ProductListResult, error) {
	result, err := s.Repo.GetProducts(page, size)
	if err != nil {
		return products.ProductListResult{}, err
	}
	// Una página sin productos se responde como lista vacía, no como null.
	if result.Products == nil {
		result.Products = []products.ProductModel{}
	}
	return result, nil
}

func (s *ProductsService) GetProduct(id uint) (products.ProductModel, error) {
	product, err := s.Repo.GetProductByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return products.ProductModel{}, ErrProductNotFound
	}
	return product, err
}

// CreateProduct da de alta un producto. Que lo pida un director lo controla el gateway.
// supplier puede ser nil: el proveedor es solo informativo.
func (s *ProductsService) CreateProduct(name, description string, supplier *string, trackID string) (products.ProductModel, error) {
	product := products.ProductModel{Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Supplier: supplier}
	if product.Name == "" {
		return products.ProductModel{}, ErrNameRequired
	}
	if err := s.Repo.CreateProduct(&product); err != nil {
		return products.ProductModel{}, err
	}
	s.Publisher.ProductUpdated(product.ID, trackID)
	return product, nil
}

// UpdateProduct cambia el nombre, la descripción y el proveedor de un producto que ya existe.
func (s *ProductsService) UpdateProduct(id uint, name, description string, supplier *string, trackID string) (products.ProductModel, error) {
	product := products.ProductModel{ID: id, Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Supplier: supplier}
	if product.Name == "" {
		return products.ProductModel{}, ErrNameRequired
	}
	if _, err := s.GetProduct(id); err != nil {
		return products.ProductModel{}, err
	}
	if err := s.Repo.UpdateProduct(product); err != nil {
		return products.ProductModel{}, err
	}
	s.Publisher.ProductUpdated(product.ID, trackID)
	return product, nil
}

// SearchProducts busca por texto en el nombre y la descripción, ordenado por relevancia o por nombre.
func (s *ProductsService) SearchProducts(query products.SearchQuery) (products.SearchResultList, error) {
	result, err := s.Search.Search(query)
	if err != nil {
		return products.SearchResultList{}, err
	}
	if result.Results == nil {
		result.Results = []products.SearchResult{}
	}
	return result, nil
}

// IndexProduct lee el producto de MySQL y reemplaza su documento en Solr. Como siempre lee el estado
// actual, no importa si el evento llegó repetido: vuelve a escribir lo mismo.
func (s *ProductsService) IndexProduct(id uint) error {
	product, err := s.GetProduct(id)
	if err != nil {
		return err
	}
	return s.Search.SaveProducts([]products.ProductModel{product})
}

// RebuildIndex crea los campos si el índice es nuevo y vuelve a escribir todos los productos de MySQL.
// Devuelve cuántos productos escribió.
func (s *ProductsService) RebuildIndex() (int, error) {
	if err := s.Search.CreateFields(); err != nil {
		return 0, err
	}
	written := 0
	for page := 1; ; page++ {
		result, err := s.Repo.GetProducts(page, rebuildPageSize)
		if err != nil {
			return written, err
		}
		if len(result.Products) == 0 {
			return written, nil
		}
		if err := s.Search.SaveProducts(result.Products); err != nil {
			return written, err
		}
		written += len(result.Products)
	}
}
