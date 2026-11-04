package reservations

import (
	"errors"
	"testing"

	"inventario/models/movements"
	"inventario/models/products"
	"inventario/models/reservations"
	"inventario/models/stock"
	productsRepo "inventario/repositories/products"
	stockRepo "inventario/repositories/stock"
)

// newService arma el Service con los Mocks. El producto 1 tiene 10 unidades en la sucursal 1 y 2 en la 2.
// El producto 2 existe pero no tiene stock en ningún lado.
func newService() (ReservationsService, stockRepo.StockChangesMock) {
	mock := stockRepo.StockChangesMock{
		Stock: map[string]stock.StockRow{
			stockRepo.StockKey(1, 1): {ProductID: 1, BranchID: 1, Total: 10},
			stockRepo.StockKey(1, 2): {ProductID: 1, BranchID: 2, Total: 2},
		},
		Reservations:    map[string]reservations.ReservationModel{},
		Movements:       &[]movements.MovementModel{},
		ProcessedEvents: map[string]bool{},
	}
	mockProducts := productsRepo.ProductsMock{
		MockDB: []products.ProductModel{{ID: 1, Name: "Harina 000"}, {ID: 2, Name: "Azúcar"}},
	}
	return ReservationsService{Repo: mock, ProductsRepo: mockProducts}, mock
}

func reservation(transferID string, quantity int) reservations.ReservationModel {
	return reservations.ReservationModel{TransferID: transferID, ProductID: 1, OriginBranchID: 1, DestinationBranchID: 2, Quantity: quantity}
}

func TestReserve_Success(t *testing.T) {
	service, mock := newService()

	result, err := service.Reserve(reservation("t-1", 4))

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if result.Status != reservations.StatusActive {
		t.Errorf("Se esperaba la reserva ACTIVA, pero quedó: %s", result.Status)
	}
	// Ejemplo de ARCHITECTURE: el origen queda 10 / 4 / 6 y el destino no cambia.
	origin := mock.Stock[stockRepo.StockKey(1, 1)]
	if origin.Total != 10 || origin.Reserved != 4 {
		t.Errorf("Se esperaba el origen en 10 total y 4 reservadas, pero quedó: %+v", origin)
	}
	if mock.Stock[stockRepo.StockKey(1, 2)].Total != 2 {
		t.Errorf("El destino no tenía que cambiar")
	}
}

// CA-02: con menos disponible que lo pedido, no se reserva nada.
func TestReserve_InsufficientStock(t *testing.T) {
	service, mock := newService()
	service.Reserve(reservation("t-1", 4))

	_, err := service.Reserve(reservation("t-2", 7))

	if !errors.Is(err, ErrInsufficientStock) {
		t.Errorf("Se esperaba ErrInsufficientStock (quedan 6 disponibles), pero se obtuvo: %v", err)
	}
	if mock.Stock[stockRepo.StockKey(1, 1)].Reserved != 4 {
		t.Errorf("La reserva rechazada no tenía que cambiar lo reservado")
	}
}

func TestReserve_ProductWithoutStock(t *testing.T) {
	service, _ := newService()
	request := reservation("t-1", 1)
	request.ProductID = 2

	_, err := service.Reserve(request)

	if !errors.Is(err, ErrInsufficientStock) {
		t.Errorf("Se esperaba ErrInsufficientStock, pero se obtuvo: %v", err)
	}
}

func TestReserve_ProductNotFound(t *testing.T) {
	service, _ := newService()
	request := reservation("t-1", 1)
	request.ProductID = 99

	_, err := service.Reserve(request)

	if !errors.Is(err, ErrProductNotFound) {
		t.Errorf("Se esperaba ErrProductNotFound, pero se obtuvo: %v", err)
	}
}

func TestReserve_SameBranch(t *testing.T) {
	service, _ := newService()
	request := reservation("t-1", 1)
	request.DestinationBranchID = 1

	_, err := service.Reserve(request)

	if !errors.Is(err, ErrInvalidReservation) {
		t.Errorf("Se esperaba ErrInvalidReservation, pero se obtuvo: %v", err)
	}
}

// Si el mismo pedido llega dos veces, la segunda no vuelve a reservar.
func TestReserve_Idempotent(t *testing.T) {
	service, mock := newService()

	service.Reserve(reservation("t-1", 4))
	_, err := service.Reserve(reservation("t-1", 4))

	if err != nil {
		t.Fatalf("El pedido repetido no tenía que fallar: %v", err)
	}
	if mock.Stock[stockRepo.StockKey(1, 1)].Reserved != 4 {
		t.Errorf("Se reservó dos veces: %+v", mock.Stock[stockRepo.StockKey(1, 1)])
	}
}

func TestRelease_ActiveReservation(t *testing.T) {
	service, mock := newService()
	service.Reserve(reservation("t-1", 4))

	result, err := service.Release("t-1")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if result.Status != reservations.StatusReleased {
		t.Errorf("Se esperaba LIBERADA, pero quedó: %s", result.Status)
	}
	// El origen vuelve a 10 / 0 / 10.
	if origin := mock.Stock[stockRepo.StockKey(1, 1)]; origin.Total != 10 || origin.Reserved != 0 {
		t.Errorf("Se esperaba el origen sin reservas, pero quedó: %+v", origin)
	}
}

// Si el pedido de liberar llega antes que el de reservar (timeout), la reserva que llega tarde se rechaza.
func TestRelease_BeforeReserve(t *testing.T) {
	service, mock := newService()

	if _, err := service.Release("t-1"); err != nil {
		t.Fatalf("Liberar una reserva que no existe no tenía que fallar: %v", err)
	}
	_, err := service.Reserve(reservation("t-1", 4))

	if !errors.Is(err, ErrReservationReleased) {
		t.Errorf("Se esperaba ErrReservationReleased, pero se obtuvo: %v", err)
	}
	if mock.Stock[stockRepo.StockKey(1, 1)].Reserved != 0 {
		t.Errorf("La reserva tardía no tenía que reservar nada")
	}
}

func TestRelease_Twice(t *testing.T) {
	service, mock := newService()
	service.Reserve(reservation("t-1", 4))

	service.Release("t-1")
	_, err := service.Release("t-1")

	if err != nil {
		t.Fatalf("Liberar dos veces no tenía que fallar: %v", err)
	}
	if mock.Stock[stockRepo.StockKey(1, 1)].Reserved != 0 {
		t.Errorf("Se liberó dos veces: %+v", mock.Stock[stockRepo.StockKey(1, 1)])
	}
}

// CA-08: cuando el traslado llega, el origen queda 6 / 0 / 6 y el destino con 6, con un movimiento en cada sucursal.
func TestConfirm_MovesStock(t *testing.T) {
	service, mock := newService()
	service.Reserve(reservation("t-1", 4))

	err := service.ConfirmByEvent("e-1", "t-1")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if origin := mock.Stock[stockRepo.StockKey(1, 1)]; origin.Total != 6 || origin.Reserved != 0 {
		t.Errorf("Se esperaba el origen en 6 sin reservas, pero quedó: %+v", origin)
	}
	if destination := mock.Stock[stockRepo.StockKey(1, 2)]; destination.Total != 6 {
		t.Errorf("Se esperaba el destino en 6, pero quedó: %+v", destination)
	}
	if mock.Reservations["t-1"].Status != reservations.StatusConfirmed {
		t.Errorf("Se esperaba la reserva CONFIRMADA, pero quedó: %s", mock.Reservations["t-1"].Status)
	}
	if len(*mock.Movements) != 2 {
		t.Fatalf("Se esperaban 2 movimientos, pero hay %d", len(*mock.Movements))
	}
	out, in := (*mock.Movements)[0], (*mock.Movements)[1]
	if out.Type != movements.TypeOut || out.BranchID != 1 || out.ResultingTotal != 6 ||
		in.Type != movements.TypeIn || in.BranchID != 2 || in.ResultingTotal != 6 {
		t.Errorf("Los movimientos no son una salida en el origen y una entrada en el destino: %+v", *mock.Movements)
	}
}

// El destino puede no tener el producto todavía: se le crea la fila.
func TestConfirm_DestinationWithoutStock(t *testing.T) {
	service, mock := newService()
	request := reservation("t-1", 3)
	request.DestinationBranchID = 3
	service.Reserve(request)

	if err := service.ConfirmByEvent("e-1", "t-1"); err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if destination := mock.Stock[stockRepo.StockKey(1, 3)]; destination.Total != 3 {
		t.Errorf("Se esperaba el destino con 3, pero quedó: %+v", destination)
	}
}

// Si RabbitMQ entrega el mismo evento dos veces, el stock se mueve una sola vez.
func TestConfirm_SameEventTwice(t *testing.T) {
	service, mock := newService()
	service.Reserve(reservation("t-1", 4))

	service.ConfirmByEvent("e-1", "t-1")
	err := service.ConfirmByEvent("e-1", "t-1")

	if err != nil {
		t.Fatalf("Confirmar dos veces no tenía que fallar: %v", err)
	}
	if mock.Stock[stockRepo.StockKey(1, 2)].Total != 6 || len(*mock.Movements) != 2 {
		t.Errorf("Se confirmó dos veces: %+v", mock.Stock)
	}
}

// Una reserva confirmada ya no se puede liberar, y una liberada ya no se puede confirmar.
func TestConfirmAndRelease_CannotCross(t *testing.T) {
	service, _ := newService()
	service.Reserve(reservation("t-1", 4))
	service.Reserve(reservation("t-2", 2))
	service.ConfirmByEvent("e-1", "t-1")
	service.Release("t-2")

	if _, err := service.Release("t-1"); !errors.Is(err, ErrReservationConfirmed) {
		t.Errorf("Se esperaba ErrReservationConfirmed, pero se obtuvo: %v", err)
	}
	if err := service.ConfirmByEvent("e-2", "t-2"); !errors.Is(err, ErrReservationReleased) {
		t.Errorf("Se esperaba ErrReservationReleased, pero se obtuvo: %v", err)
	}
}

func TestConfirm_NotFound(t *testing.T) {
	service, _ := newService()

	err := service.ConfirmByEvent("e-1", "no-existe")

	if !errors.Is(err, ErrReservationNotFound) {
		t.Errorf("Se esperaba ErrReservationNotFound, pero se obtuvo: %v", err)
	}
}

// Aunque llegaran dos eventos distintos para la misma transferencia, la reserva confirmada no se confirma de nuevo.
func TestConfirm_TwoEventsSameTransfer(t *testing.T) {
	service, mock := newService()
	service.Reserve(reservation("t-1", 4))

	service.ConfirmByEvent("e-1", "t-1")
	err := service.ConfirmByEvent("e-2", "t-1")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if mock.Stock[stockRepo.StockKey(1, 2)].Total != 6 || len(*mock.Movements) != 2 {
		t.Errorf("Se confirmó dos veces: %+v", mock.Stock)
	}
}

// CA-10: cuando se cancela el traslado, las unidades vuelven al disponible del origen y el evento queda anotado.
func TestReleaseByEvent_Success(t *testing.T) {
	service, mock := newService()
	service.Reserve(reservation("t-1", 4))

	err := service.ReleaseByEvent("e-1", "t-1")

	if err != nil {
		t.Fatalf("No se esperaba un error, pero falló: %v", err)
	}
	if origin := mock.Stock[stockRepo.StockKey(1, 1)]; origin.Total != 10 || origin.Reserved != 0 {
		t.Errorf("Se esperaba el origen en 10 sin reservas, pero quedó: %+v", origin)
	}
	if !mock.ProcessedEvents["e-1"] {
		t.Errorf("El evento tenía que quedar anotado como procesado")
	}
}

// Si no se pudo procesar, el evento no queda anotado: va a la cola de errores y se puede volver a procesar.
func TestConfirmByEvent_FailureNotRecorded(t *testing.T) {
	service, mock := newService()

	service.ConfirmByEvent("e-1", "no-existe")

	if mock.ProcessedEvents["e-1"] {
		t.Errorf("Un evento que falló no tenía que quedar anotado")
	}
}
