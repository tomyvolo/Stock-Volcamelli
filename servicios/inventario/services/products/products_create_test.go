package products

import (
	"errors"
	"testing"

	"inventario/models/products"
	repo "inventario/repositories/products"
)

// publisherMock anota los productos que se publicaron.
type publisherMock struct {
	published *[]uint
}

func (p publisherMock) ProductUpdated(productID uint, trackID string) {
	*p.published = append(*p.published, productID)
}

func newPublisher() (publisherMock, *[]uint) {
	published := &[]uint{}
	return publisherMock{published: published}, published
}

func TestCreateProduct_Success(t *testing.T) {
	publisher, published := newPublisher()
	service := ProductsService{Repo: repo.ProductsMock{MockDB: []products.ProductModel{{ID: 1, Name: "Harina 000"}}}, Publisher: publisher}

	product, err := service.CreateProduct("  Polenta ", "Paquete de 500 g", nil, "abc")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if product.ID != 2 || product.Name != "Polenta" || product.Supplier != nil {
		t.Errorf("Se esperaba el producto 2 sin espacios de más y sin proveedor, pero se obtuvo: %+v", product)
	}
	if len(*published) != 1 || (*published)[0] != 2 {
		t.Errorf("Se esperaba publicar ProductoActualizado del producto 2, pero se publicó: %v", *published)
	}
}

func TestCreateProduct_EmptyName(t *testing.T) {
	publisher, published := newPublisher()
	service := ProductsService{Repo: repo.ProductsMock{}, Publisher: publisher}

	_, err := service.CreateProduct("   ", "Sin nombre", nil, "abc")

	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("Se esperaba ErrNameRequired, pero se obtuvo: %v", err)
	}
	if len(*published) != 0 {
		t.Errorf("Si no se creó, no se tenía que publicar nada")
	}
}

func TestUpdateProduct_Success(t *testing.T) {
	publisher, published := newPublisher()
	mock := repo.ProductsMock{MockDB: []products.ProductModel{{ID: 1, Name: "Harina 000"}}}
	service := ProductsService{Repo: mock, Publisher: publisher}
	supplier := "Molinos del Sur"

	_, err := service.UpdateProduct(1, "Harina 0000", "Paquete de 1 kg", &supplier, "abc")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	product, _ := service.GetProduct(1)
	if product.Name != "Harina 0000" || product.Supplier == nil || *product.Supplier != supplier {
		t.Errorf("El producto no cambió: %+v", product)
	}
	if len(*published) != 1 || (*published)[0] != 1 {
		t.Errorf("Se esperaba publicar ProductoActualizado del producto 1, pero se publicó: %v", *published)
	}
}

func TestUpdateProduct_NotFound(t *testing.T) {
	publisher, _ := newPublisher()
	service := ProductsService{Repo: repo.ProductsMock{}, Publisher: publisher}

	_, err := service.UpdateProduct(99, "Otro", "", nil, "abc")

	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("Se esperaba ErrProductNotFound, pero se obtuvo: %v", err)
	}
}
