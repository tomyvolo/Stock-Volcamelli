package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// proveedor levanta un servidor que responde como la API compatible con OpenAI y anota lo que recibe.
func proveedor(t *testing.T, codigo int, respuesta string, recibido *map[string]any, clave *string) OpenAI {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		*clave = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(recibido)
		w.WriteHeader(codigo)
		fmt.Fprint(w, respuesta)
	}))
	t.Cleanup(servidor.Close)
	return OpenAI{BaseURL: servidor.URL, Model: "openai/gpt-oss-120b", APIKey: "clave-del-proveedor", HTTP: servidor.Client()}
}

func TestComplete_PideUnaHerramienta(t *testing.T) {
	var recibido map[string]any
	var clave string
	o := proveedor(t, http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":null,
		"tool_calls":[{"id":"call_1","type":"function","function":{"name":"consultar_stock","arguments":"{\"branchId\":1}"}}]}}]}`, &recibido, &clave)

	answer, err := o.Complete(context.Background(),
		[]Message{{Role: "user", Content: "¿Cuánto stock hay en Centro?"}},
		[]Tool{{Name: "consultar_stock", Description: "Stock de una sucursal", Parameters: json.RawMessage(`{"type":"object"}`)}})

	if err != nil || len(answer.ToolCalls) != 1 || answer.ToolCalls[0].Function.Name != "consultar_stock" ||
		answer.ToolCalls[0].Function.Arguments != `{"branchId":1}` {
		t.Fatalf("Se esperaba el pedido de consultar_stock, pero se obtuvo %+v (%v)", answer, err)
	}
	if clave != "Bearer clave-del-proveedor" || recibido["model"] != "openai/gpt-oss-120b" {
		t.Errorf("Se esperaba la clave y el modelo configurados, pero llegó %q y %v", clave, recibido["model"])
	}
	herramientas, _ := recibido["tools"].([]any)
	if len(herramientas) != 1 {
		t.Fatalf("Se esperaba mandar la herramienta, pero se mandó %v", recibido["tools"])
	}
	funcion := herramientas[0].(map[string]any)["function"].(map[string]any)
	if funcion["name"] != "consultar_stock" || funcion["parameters"] == nil {
		t.Errorf("La herramienta tenía que ir con su nombre y sus parámetros, pero fue %v", funcion)
	}
}

// Sin clave válida, con el límite del plan gratis o con el proveedor caído, el modelo no está disponible.
func TestComplete_ProveedorConError(t *testing.T) {
	for _, codigo := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		var recibido map[string]any
		var clave string
		o := proveedor(t, codigo, `{"error":{"message":"algo"}}`, &recibido, &clave)

		_, err := o.Complete(context.Background(), []Message{{Role: "user", Content: "hola"}}, nil)

		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("Con %d se esperaba ErrUnavailable, pero se obtuvo %v", codigo, err)
		}
	}
}
