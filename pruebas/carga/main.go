// Generador de carga para las pruebas de D13 (docs/adr/ADR-013-capacidad-y-costos.md).
// Manda pedidos GET al gateway con un worker pool durante un tiempo fijo y al final
// informa cuántos pedidos hizo, cuántos fallaron, pedidos por segundo y percentiles de duración.
//
// Uso:
//
//	TOKEN=<jwt> go run . -url http://localhost:8080/api/branches/1/stock -workers 50 -duracion 20s
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type resultado struct {
	status   int // 0 si no hubo respuesta (timeout o error de red)
	duracion time.Duration
}

func main() {
	url := flag.String("url", "http://localhost:8080/api/branches/1/stock", "ruta a consultar, siempre por el gateway")
	workers := flag.Int("workers", 10, "cantidad de pedidos en paralelo")
	duracion := flag.Duration("duracion", 20*time.Second, "cuánto dura la prueba")
	timeout := flag.Duration("timeout", 2*time.Second, "timeout de cada pedido, igual al del gateway")
	flag.Parse()

	token := os.Getenv("TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "falta la variable de entorno TOKEN (hacer login y copiar el token)")
		os.Exit(1)
	}

	// Un solo cliente con suficientes conexiones abiertas para todos los workers.
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: *workers}}

	pedidos := make(chan struct{})
	resultados := make(chan resultado, *workers)

	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range pedidos {
				resultados <- hacerPedido(client, *url, token, *timeout)
			}
		}()
	}

	// Reparte pedidos hasta que se termina el tiempo; cada worker toma el siguiente cuando termina el suyo.
	inicio := time.Now()
	go func() {
		fin := time.After(*duracion)
		for {
			select {
			case <-fin:
				close(pedidos)
				return
			case pedidos <- struct{}{}:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(resultados)
	}()

	var duraciones []time.Duration
	porStatus := map[int]int{}
	for r := range resultados {
		duraciones = append(duraciones, r.duracion)
		porStatus[r.status]++
	}
	total := time.Since(inicio)

	informar(*url, *workers, total, duraciones, porStatus)
}

func hacerPedido(client *http.Client, url, token string, timeout time.Duration) resultado {
	// Si un pedido se cuelga, el contexto lo corta y no frena la prueba.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return resultado{}
	}
	req.Header.Set("Authorization", "Bearer "+token)

	inicio := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return resultado{duracion: time.Since(inicio)}
	}
	// Leemos el cuerpo entero para medir la respuesta completa y reusar la conexión.
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resultado{status: resp.StatusCode, duracion: time.Since(inicio)}
}

func informar(url string, workers int, total time.Duration, duraciones []time.Duration, porStatus map[int]int) {
	n := len(duraciones)
	if n == 0 {
		fmt.Println("no se hizo ningún pedido")
		return
	}
	sort.Slice(duraciones, func(i, j int) bool { return duraciones[i] < duraciones[j] })
	percentil := func(p float64) time.Duration { return duraciones[int(float64(n-1)*p)] }

	errores := 0
	for status, cantidad := range porStatus {
		if status < 200 || status >= 300 {
			errores += cantidad
		}
	}

	fmt.Printf("url=%s workers=%d duracion=%.1fs\n", url, workers, total.Seconds())
	fmt.Printf("pedidos=%d por_segundo=%.1f errores=%d (%.2f%%)\n", n, float64(n)/total.Seconds(), errores, 100*float64(errores)/float64(n))
	fmt.Printf("p50=%dms p95=%dms p99=%dms max=%dms\n",
		percentil(0.50).Milliseconds(), percentil(0.95).Milliseconds(), percentil(0.99).Milliseconds(), duraciones[n-1].Milliseconds())
	fmt.Printf("por_status=%v (0 = sin respuesta)\n", porStatus)
}
