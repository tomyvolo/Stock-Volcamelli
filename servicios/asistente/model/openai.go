// Package model habla con el modelo de lenguaje usando la API compatible con OpenAI, que ofrecen
// Groq, Gemini y la mayoría de los proveedores (ADR-018). Cambiar de proveedor es cambiar la URL,
// el modelo y la clave, sin tocar código.
package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Timeout es lo que esperamos cada respuesta del modelo. Una pregunta puede dar varias vueltas y el
// gateway espera 30 segundos en total.
const Timeout = 15 * time.Second

// ErrUnavailable es que el modelo no respondió o respondió con error: el chat dice que el asistente no está disponible.
var ErrUnavailable = errors.New("el modelo no está disponible")

// Message es un mensaje de la conversación, en el formato de la API.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall es una herramienta que el modelo pide ejecutar. Arguments es un JSON en texto.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool es una herramienta que el modelo puede pedir, con el esquema JSON de sus parámetros.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// OpenAI es el cliente de la API /chat/completions.
type OpenAI struct {
	BaseURL string
	Model   string
	APIKey  string
	HTTP    *http.Client
}

type toolJSON struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

// Complete le manda la conversación y las herramientas al modelo, y devuelve su respuesta: un texto
// o los pedidos de herramientas.
func (o OpenAI) Complete(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	payload := struct {
		Model       string     `json:"model"`
		Messages    []Message  `json:"messages"`
		Tools       []toolJSON `json:"tools,omitempty"`
		Temperature float64    `json:"temperature"`
	}{Model: o.Model, Messages: messages, Temperature: 0}
	for _, tool := range tools {
		var t toolJSON
		t.Type = "function"
		t.Function.Name, t.Function.Description, t.Function.Parameters = tool.Name, tool.Description, tool.Parameters
		payload.Tools = append(payload.Tools, t)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+o.APIKey)
	request.Header.Set("User-Agent", "stock-volcamelli-asistente")

	response, err := o.HTTP.Do(request)
	if err != nil {
		return Message{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		// 401 por la clave, 429 por el límite del plan gratis o 5xx: para el usuario es lo mismo.
		return Message{}, fmt.Errorf("%w: respondió %d: %.300s", ErrUnavailable, response.StatusCode, raw)
	}

	var parsed struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
		return Message{}, fmt.Errorf("%w: respuesta ilegible: %.300s", ErrUnavailable, raw)
	}
	return parsed.Choices[0].Message, nil
}
