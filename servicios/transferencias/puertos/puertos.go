// Package puertos tiene las interfaces que el centro necesita de afuera. Las define el centro,
// y cada una tiene un adaptador que la implementa (en adaptadores/).
package puertos

import (
	"errors"
	"time"

	"transferencias/dominio"
)

// Errores que devuelven los adaptadores, para que el centro decida sin conocer HTTP ni Rumbo.
var (
	ErrNoExiste         = errors.New("no existe")
	ErrSinStock         = errors.New("no hay stock disponible suficiente")
	ErrNoDisponible     = errors.New("el servicio no respondió a tiempo o no está disponible")
	ErrRechazoDelPedido = errors.New("el transporte rechazó el pedido")
	ErrSinLugar         = errors.New("la franja ya no tiene lugar")
	ErrEstadoNoPermite  = errors.New("el estado del traslado no permite la operación")
)

// Inventario reserva y libera unidades. Lo usamos por HTTP, a través del NGINX de Inventario.
type Inventario interface {
	// NombreProducto devuelve ErrNoExiste si el producto no existe.
	NombreProducto(productoID uint, trackID string) (string, error)
	// Reservar devuelve ErrSinStock si no alcanza el disponible, o ErrNoDisponible si no respondió:
	// en ese caso no sabemos si llegó a reservar.
	Reservar(t *dominio.Transferencia, trackID string) error
	Liberar(transferenciaID, trackID string) error
}

// Sucursales trae el nombre y la dirección de una sucursal. Devuelve ErrNoExiste si no existe.
type Sucursales interface {
	Obtener(sucursalID uint, trackID string) (dominio.Ubicacion, error)
}

// Transporte reserva y sigue los traslados. Lo llamamos Transporte y no Rumbo a propósito: si cambiara
// el proveedor, se escribe otro adaptador y el centro queda igual. Cada operación devuelve lo que
// informó el transporte, ya traducido a una Novedad.
type Transporte interface {
	// Franjas devuelve, en orden, las franjas del día de fecha que todavía tienen lugar.
	Franjas(fecha time.Time) ([]dominio.Franja, error)
	// Reservar reserva el traslado en t.Franja. Devuelve ErrSinLugar si la franja ya no tiene lugar o
	// ya es tarde para reservarla (hay que elegir otra), ErrRechazoDelPedido (con el motivo en la novedad)
	// si el pedido es inválido, o ErrNoDisponible si hay que reintentar con la misma franja.
	Reservar(t *dominio.Transferencia) (dominio.Novedad, error)
	// Cambios devuelve lo que cambió desde la fecha indicada y la fecha hasta la que leyó.
	Cambios(desde time.Time) ([]dominio.Novedad, time.Time, error)
	Consultar(t *dominio.Transferencia) (dominio.Novedad, error)
	// Cancelar devuelve ErrEstadoNoPermite si el traslado ya está en curso o terminó.
	Cancelar(t *dominio.Transferencia, motivo string) (dominio.Novedad, error)
}

// Repositorio guarda las transferencias.
type Repositorio interface {
	Guardar(t *dominio.Transferencia) error
	// Obtener devuelve ErrNoExiste si no hay una transferencia con ese id.
	Obtener(id string) (*dominio.Transferencia, error)
	Listar(filtro Filtro) ([]*dominio.Transferencia, int64, error)
	// PendientesDeEnvio son las que hay que mandarle al transporte: sin reservar, con una cancelación
	// por mandar, o con una reserva por liberar.
	PendientesDeEnvio() ([]*dominio.Transferencia, error)
	// UltimaConsulta es hasta dónde leímos los cambios del transporte.
	UltimaConsulta() (time.Time, error)
	GuardarUltimaConsulta(hasta time.Time) error
}

// Filtro de la lista de transferencias. Los campos vacíos no filtran.
type Filtro struct {
	SucursalID uint // que salgan de la sucursal o lleguen a ella
	Estado     dominio.Estado
	Pagina     int
	Tamanio    int
}

// Evento es lo que se publica cuando una transferencia cambia: Inventario confirma o libera la reserva
// con TrasladoCompletado y TrasladoCancelado, y Tiempo Real avisa al frontend con TransferenciaActualizada.
type Evento struct {
	ID              string
	Tipo            string
	TransferenciaID string
	Estado          dominio.Estado
	// Con el origen y el destino, la pantalla de una sucursal sabe si el aviso le toca.
	OrigenID  uint
	DestinoID uint
	Motivo    string
	TrackID   string
}

const (
	TrasladoCompletado       = "TrasladoCompletado"
	TrasladoCancelado        = "TrasladoCancelado"
	TransferenciaActualizada = "TransferenciaActualizada"
)

// Eventos publica los eventos de las transferencias.
type Eventos interface {
	Publicar(evento Evento) error
}

// Registro escribe los logs. nivel es info, warn o error.
type Registro interface {
	Escribir(trackID, nivel, mensaje string)
}
