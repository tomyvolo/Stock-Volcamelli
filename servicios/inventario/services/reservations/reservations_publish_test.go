package reservations

import (
	"testing"
)

// published es un aviso de StockActualizado que anotó el publicador de prueba.
type published struct {
	BranchID, ProductID uint
	Available           int
}

// publisherMock anota los avisos en lugar de mandarlos a RabbitMQ.
type publisherMock struct{ sent *[]published }

func (p publisherMock) StockUpdated(branchID, productID uint, available int, trackID string) {
	*p.sent = append(*p.sent, published{branchID, productID, available})
}

func newPublishingService() (ReservationsService, *[]published) {
	service, _ := newService()
	sent := &[]published{}
	service.Publisher = publisherMock{sent}
	return service, sent
}

// Reservar 4 de 10 deja 6 disponibles en el origen, y se avisa solo el origen.
func TestReserve_PublishesOrigin(t *testing.T) {
	service, sent := newPublishingService()

	if _, err := service.Reserve(reservation("t-1", 4)); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}

	if len(*sent) != 1 || (*sent)[0] != (published{1, 1, 6}) {
		t.Errorf("Se esperaba un aviso de 6 disponibles en la sucursal 1, pero se mandó: %+v", *sent)
	}
}

// Si la reserva se rechaza, el stock no cambió y no se avisa nada.
func TestReserve_RejectedDoesNotPublish(t *testing.T) {
	service, sent := newPublishingService()

	service.Reserve(reservation("t-1", 50))

	if len(*sent) != 0 {
		t.Errorf("No se tenía que avisar nada, pero se mandó: %+v", *sent)
	}
}

// El mismo pedido de reserva dos veces avisa una sola vez, porque la segunda no cambia nada.
func TestReserve_RepeatedPublishesOnce(t *testing.T) {
	service, sent := newPublishingService()

	service.Reserve(reservation("t-1", 4))
	service.Reserve(reservation("t-1", 4))

	if len(*sent) != 1 {
		t.Errorf("Se esperaba un solo aviso, pero se mandaron: %+v", *sent)
	}
}

// Liberar devuelve las 4 al disponible del origen.
func TestRelease_PublishesOrigin(t *testing.T) {
	service, sent := newPublishingService()
	service.Reserve(reservation("t-1", 4))

	if _, err := service.Release("t-1"); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}

	if len(*sent) != 2 || (*sent)[1] != (published{1, 1, 10}) {
		t.Errorf("Se esperaba un aviso de 10 disponibles en la sucursal 1, pero se mandó: %+v", *sent)
	}
}

// Liberar una transferencia sin reserva no cambia el stock: no se avisa.
func TestRelease_WithoutReservationDoesNotPublish(t *testing.T) {
	service, sent := newPublishingService()

	service.Release("t-sin-reserva")

	if len(*sent) != 0 {
		t.Errorf("No se tenía que avisar nada, pero se mandó: %+v", *sent)
	}
}

// Confirmar avisa las dos sucursales: el origen sigue con 6 disponibles y el destino pasa de 2 a 6.
func TestConfirmByEvent_PublishesBothBranches(t *testing.T) {
	service, sent := newPublishingService()
	service.Reserve(reservation("t-1", 4))

	if err := service.ConfirmByEvent("e-1", "t-1"); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}

	got := (*sent)[1:]
	if len(got) != 2 || got[0] != (published{1, 1, 6}) || got[1] != (published{2, 1, 6}) {
		t.Errorf("Se esperaban avisos de 6 en la sucursal 1 y 6 en la 2, pero se mandó: %+v", got)
	}
}

// Un evento repetido ya está procesado: no cambia el stock y no se avisa de nuevo.
func TestConfirmByEvent_RepeatedPublishesOnce(t *testing.T) {
	service, sent := newPublishingService()
	service.Reserve(reservation("t-1", 4))

	service.ConfirmByEvent("e-1", "t-1")
	service.ConfirmByEvent("e-1", "t-1")

	if len(*sent) != 3 {
		t.Errorf("Se esperaban 3 avisos (reserva y las dos sucursales), pero se mandaron: %+v", *sent)
	}
}

// Liberar por evento avisa el origen, igual que liberar por HTTP.
func TestReleaseByEvent_PublishesOrigin(t *testing.T) {
	service, sent := newPublishingService()
	service.Reserve(reservation("t-1", 4))

	if err := service.ReleaseByEvent("e-1", "t-1"); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}

	if len(*sent) != 2 || (*sent)[1] != (published{1, 1, 10}) {
		t.Errorf("Se esperaba un aviso de 10 disponibles en la sucursal 1, pero se mandó: %+v", *sent)
	}
}
