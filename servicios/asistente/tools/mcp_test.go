package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type stockInput struct {
	BranchID uint `json:"branchId"`
}

// servidorMCP levanta un servidor MCP chico, como el nuestro (sin sesión y con respuestas JSON), con una
// herramienta que devuelve el token con el que la llamaron, o un error si no hay token.
func servidorMCP(t *testing.T) string {
	server := mcp.NewServer(&mcp.Implementation{Name: "prueba", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "consultar_stock", Description: "Stock de una sucursal"},
		func(_ context.Context, request *mcp.CallToolRequest, in stockInput) (*mcp.CallToolResult, any, error) {
			token := request.Extra.Header.Get("Authorization")
			if in.BranchID == 9 {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "No existe esa sucursal."}}}, nil, nil
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: token + "|" + request.Extra.Header.Get("X-Track-ID")}}}, nil, nil
		})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

func TestSession_ListaYLlamaConElToken(t *testing.T) {
	session, err := Client{Endpoint: servidorMCP(t)}.Open(context.Background(), "Bearer token-del-usuario", "track-1")
	if err != nil {
		t.Fatalf("No se pudo conectar: %v", err)
	}
	defer session.Close()

	list, err := session.List(context.Background())
	if err != nil || len(list) != 1 || list[0].Name != "consultar_stock" || !json.Valid(list[0].Parameters) {
		t.Fatalf("Se esperaba la herramienta con su esquema, pero se obtuvo %+v (%v)", list, err)
	}

	text, isError, err := session.Call(context.Background(), "consultar_stock", json.RawMessage(`{"branchId":1}`))
	if err != nil || isError || text != "Bearer token-del-usuario|track-1" {
		t.Errorf("La herramienta tenía que recibir el token y el TrackID, pero respondió %q (%v, %v)", text, isError, err)
	}
}

func TestSession_ErrorDeLaHerramienta(t *testing.T) {
	session, _ := Client{Endpoint: servidorMCP(t)}.Open(context.Background(), "Bearer token", "")
	defer session.Close()

	text, isError, err := session.Call(context.Background(), "consultar_stock", json.RawMessage(`{"branchId":9}`))

	if err != nil || !isError || text != "No existe esa sucursal." {
		t.Errorf("Se esperaba el error de la herramienta, pero se obtuvo %q (%v, %v)", text, isError, err)
	}
}

func TestOpen_ServidorCaido(t *testing.T) {
	_, err := Client{Endpoint: "http://127.0.0.1:1/mcp"}.Open(context.Background(), "Bearer token", "")

	if err == nil {
		t.Error("Se esperaba un error al conectar con un servidor MCP que no está")
	}
}
