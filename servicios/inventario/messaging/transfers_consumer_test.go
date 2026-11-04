package messaging

import (
	"errors"
	"testing"

	"inventario/models/movements"
	"inventario/models/products"
	"inventario/models/reservations"
	"inventario/models/stock"
	productsRepo "inventario/repositories/products"
	stockRepo "inventario/repositories/stock"
	services "inventario/services/reservations"
)

// newConsumer arma el consumidor con los Mocks: la transferencia t-1 tiene reservadas 4 de las 10 unidades
// del producto 1 en la sucursal 1, para llevarlas a la 2. No hace falta RabbitMQ: probamos Process.
func newConsumer() (TransfersConsumer, stockRepo.StockChangesMock) {
	mock := stockRepo.StockChangesMock{
		Stock: map[string]stock.StockRow{
			stockRepo.StockKey(1, 1): {ProductID: 1, BranchID: 1, Total: 10, Reserved: 4},
			stockRepo.StockKey(1, 2): {ProductID: 1, BranchID: 2, Total: 2},
		},
		Reservations: map[string]reservations.ReservationModel{
			"t-1": {TransferID: "t-1", ProductID: 1, OriginBranchID: 1, DestinationBranchID: 2, Quantity: 4, Status: reservations.StatusActive},
		},
		Movements:       &[]movements.MovementModel{},
		ProcessedEvents: map[string]bool{},
	}
	mockProducts := productsRepo.ProductsMock{MockDB: []products.ProductModel{{ID: 1, Name: "Harina 000"}}}
	service := services.ReservationsService{Repo: mock, ProductsRepo: mockProducts}
	return TransfersConsumer{Service: service}, mock
}

func TestProcess_Completed(t *testing.T) {
	consumer, mock := newConsumer()

	trackID, err := consumer.Process(reservations.EventTransferCompleted,
		[]byte(`{"id":"e-1","tipo":"TrasladoCompletado","transferenciaId":"t-1","estado":"COMPLETADA","trackId":"abc"}`))

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if trackID != "abc" {
		t.Errorf("Se esperaba el TrackID abc, pero se obtuvo: %q", trackID)
	}
	if mock.Stock[stockRepo.StockKey(1, 2)].Total != 6 || mock.Reservations["t-1"].Status != reservations.StatusConfirmed {
		t.Errorf("La reserva no se confirmó: %+v", mock.Stock)
	}
}

func TestProcess_Cancelled(t *testing.T) {
	consumer, mock := newConsumer()

	_, err := consumer.Process(reservations.EventTransferCancelled,
		[]byte(`{"id":"e-1","tipo":"TrasladoCancelado","transferenciaId":"t-1","estado":"RECHAZADA","motivo":"sin capacidad"}`))

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if origin := mock.Stock[stockRepo.StockKey(1, 1)]; origin.Reserved != 0 {
		t.Errorf("La reserva no se liberó: %+v", origin)
	}
}

// Un mensaje que no se puede leer va a la cola de errores: reintentarlo no lo arregla.
func TestProcess_Unreadable(t *testing.T) {
	consumer, _ := newConsumer()

	for _, body := range []string{`no es json`, `{"id":"e-1"}`, `{"transferenciaId":"t-1"}`} {
		_, err := consumer.Process(reservations.EventTransferCompleted, []byte(body))
		if !errors.Is(err, ErrUnreadableEvent) || !Discard(err) {
			t.Errorf("%s: se esperaba ErrUnreadableEvent y descartarlo, pero se obtuvo: %v", body, err)
		}
	}
}

// Completar una transferencia sin reserva no tiene arreglo: va a la cola de errores.
func TestProcess_ReservationNotFound(t *testing.T) {
	consumer, _ := newConsumer()

	_, err := consumer.Process(reservations.EventTransferCompleted, []byte(`{"id":"e-1","transferenciaId":"no-existe"}`))

	if !errors.Is(err, services.ErrReservationNotFound) || !Discard(err) {
		t.Errorf("Se esperaba ErrReservationNotFound y descartarlo, pero se obtuvo: %v", err)
	}
}

// Un error de la base, en cambio, se reintenta.
func TestDiscard_OtherErrors(t *testing.T) {
	if Discard(errors.New("MySQL no respondió")) {
		t.Errorf("Un error pasajero no se tenía que descartar")
	}
}
