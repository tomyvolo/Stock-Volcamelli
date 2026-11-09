// Package procesos son los adaptadores de entrada que corren solos, en segundo plano: cada tanto
// llaman a los casos de uso que mandan lo pendiente y que consultan los cambios de Rumbo.
// Por eso Transferencias corre en una sola instancia: con dos, los dos harían lo mismo.
package procesos

import (
	"time"

	"transferencias/aplicacion"
)

// Arrancar lanza los dos procesos. envio es cada cuánto se manda lo pendiente; consulta, cada cuánto
// se le preguntan los cambios a Rumbo (su contrato pide no más de una vez por minuto).
func Arrancar(servicio *aplicacion.Servicio, envio, consulta time.Duration) {
	go repetir(envio, servicio.EnviarPendientes)
	go repetir(consulta, servicio.ConsultarCambios)
}

// repetir corre la tarea cada intervalo. Si una vuelta tarda más que el intervalo, la siguiente
// espera a que termine: nunca corren dos a la vez.
func repetir(intervalo time.Duration, tarea func()) {
	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()
	for range ticker.C {
		tarea()
	}
}
