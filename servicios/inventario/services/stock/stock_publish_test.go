package stock

import (
	"testing"

	"inventario/models/movements"
)

// published es un aviso de StockActualizado que anotó el publicador de prueba.
type published struct {
	BranchID, ProductID uint
	Available           int
	TrackID             string
}

// publisherMock anota los avisos en lugar de mandarlos a RabbitMQ.
type publisherMock struct{ sent *[]published }

func (p publisherMock) StockUpdated(branchID, productID uint, available int, trackID string) {
	*p.sent = append(*p.sent, published{branchID, productID, available, trackID})
}

// Con 10 en total y 4 reservadas, una entrada de 5 deja 11 disponibles, y el aviso lleva el TrackID del pedido.
func TestAdjust_PublishesAvailable(t *testing.T) {
	service, _ := newAdjustService()
	sent := &[]published{}
	service.Publisher = publisherMock{sent}

	if _, err := service.Adjust("DIRECTOR", nil, 3, 1, movements.TypeIn, 5, "Llegó mercadería", "track-1"); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}

	if len(*sent) != 1 || (*sent)[0] != (published{3, 1, 11, "track-1"}) {
		t.Errorf("Se esperaba un aviso de 11 disponibles en la sucursal 3, pero se mandó: %+v", *sent)
	}
}

// Un ajuste rechazado no cambió el stock: no se avisa.
func TestAdjust_RejectedDoesNotPublish(t *testing.T) {
	service, _ := newAdjustService()
	sent := &[]published{}
	service.Publisher = publisherMock{sent}

	service.Adjust("DIRECTOR", nil, 3, 1, movements.TypeOut, 7, "Rotura", "")

	if len(*sent) != 0 {
		t.Errorf("No se tenía que avisar nada, pero se mandó: %+v", *sent)
	}
}
