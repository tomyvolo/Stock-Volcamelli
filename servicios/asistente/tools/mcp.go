// Package tools se conecta al servidor MCP para listar y ejecutar las herramientas. Cada conexión lleva el
// token del usuario que está chateando: el asistente no tiene permisos propios (ADR-018).
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"asistente/model"
)

// Timeout es lo que esperamos cada respuesta del servidor MCP, que a su vez espera 6 segundos al gateway.
const Timeout = 7 * time.Second

// ErrUnavailable es que el servidor MCP no respondió.
var ErrUnavailable = errors.New("el servidor MCP no está disponible")

// Session es una conexión al servidor MCP para una pregunta.
type Session interface {
	List(ctx context.Context) ([]model.Tool, error)
	// Call ejecuta una herramienta y devuelve su texto. isError es que la herramienta respondió con un
	// error pensado para el usuario (por ejemplo, sin permiso o sin stock): el modelo lo tiene que leer.
	Call(ctx context.Context, name string, arguments json.RawMessage) (text string, isError bool, err error)
	Close()
}

// Client abre sesiones contra el servidor MCP.
type Client struct {
	Endpoint string
}

// Open se conecta con el token del usuario y el TrackID del pedido.
func (c Client) Open(ctx context.Context, authorization, trackID string) (Session, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "asistente", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             c.Endpoint,
		HTTPClient:           &http.Client{Timeout: Timeout, Transport: headers{authorization: authorization, trackID: trackID}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return mcpSession{session}, nil
}

// headers agrega el token y el TrackID a cada pedido al servidor MCP. El token nunca entra en la
// conversación con el modelo.
type headers struct {
	authorization string
	trackID       string
}

func (h headers) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", h.authorization)
	if h.trackID != "" {
		clone.Header.Set("X-Track-ID", h.trackID)
	}
	return http.DefaultTransport.RoundTrip(clone)
}

type mcpSession struct {
	session *mcp.ClientSession
}

func (s mcpSession) List(ctx context.Context) ([]model.Tool, error) {
	result, err := s.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	var list []model.Tool
	for _, tool := range result.Tools {
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, err
		}
		list = append(list, model.Tool{Name: tool.Name, Description: tool.Description, Parameters: schema})
	}
	return list, nil
}

func (s mcpSession) Call(ctx context.Context, name string, arguments json.RawMessage) (string, bool, error) {
	var args map[string]any
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			// El modelo armó mal los parámetros: se lo decimos para que lo corrija.
			return "Los parámetros no son un JSON válido: " + err.Error(), true, nil
		}
	}
	result, err := s.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", false, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if t, ok := content.(*mcp.TextContent); ok {
			text.WriteString(t.Text)
		}
	}
	return text.String(), result.IsError, nil
}

func (s mcpSession) Close() { s.session.Close() }
