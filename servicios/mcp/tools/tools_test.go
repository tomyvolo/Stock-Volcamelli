package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeGateway anota los pedidos y responde lo que le digan.
type fakeGateway struct {
	calls  []recorded
	status int
	body   string
	err    error
}

type recorded struct {
	method, path, authorization string
	body                        any
}

func (f *fakeGateway) Do(_ context.Context, method, path, authorization string, body any) (int, []byte, error) {
	f.calls = append(f.calls, recorded{method, path, authorization, body})
	if f.err != nil {
		return 0, nil, f.err
	}
	return f.status, []byte(f.body), nil
}

// fakeLogs anota los mensajes, para revisar qué queda en los logs.
type fakeLogs struct{ messages []string }

func (f *fakeLogs) Write(_, _, _, message string) { f.messages = append(f.messages, message) }

const token = "Bearer token-del-usuario"

// connect levanta el servidor por HTTP, como en producción, y se conecta como lo hace el Asistente.
func connect(t *testing.T, gateway *fakeGateway, logs *fakeLogs, authorization string) *mcp.ClientSession {
	t.Helper()
	server := NewServer(Toolbox{Gateway: gateway, Logs: logs})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL,
		HTTPClient:           &http.Client{Transport: withAuthorization{authorization}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("No se pudo conectar: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

type withAuthorization struct{ value string }

func (a withAuthorization) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	if a.value != "" {
		clone.Header.Set("Authorization", a.value)
	}
	return http.DefaultTransport.RoundTrip(clone)
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("No se pudo llamar a %s: %v", name, err)
	}
	return result
}

func text(result *mcp.CallToolResult) string {
	return result.Content[0].(*mcp.TextContent).Text
}

func TestListTools_HasTheSeven(t *testing.T) {
	session := connect(t, &fakeGateway{}, &fakeLogs{}, token)

	result, err := session.ListTools(context.Background(), nil)

	if err != nil {
		t.Fatalf("No se pudieron listar: %v", err)
	}
	want := map[string]bool{"buscar_producto": true, "listar_sucursales": true, "usuarios_sucursal": true, "consultar_stock": true,
		"stock_bajo": true, "crear_transferencia": true, "estado_transferencia": true}
	if len(result.Tools) != len(want) {
		t.Errorf("Se esperaban %d herramientas, pero hay %d", len(want), len(result.Tools))
	}
	for _, tool := range result.Tools {
		if !want[tool.Name] {
			t.Errorf("Herramienta inesperada: %s", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("%s no tiene descripción, y el modelo la necesita para elegirla", tool.Name)
		}
	}
}

// Cada herramienta llama a la ruta que le corresponde, con el token del usuario.
func TestTools_CallTheRightRoute(t *testing.T) {
	cases := []struct {
		tool   string
		args   map[string]any
		method string
		path   string
	}{
		{"buscar_producto", map[string]any{"query": "harina 000"}, "GET", "/api/products/search?q=harina+000"},
		{"listar_sucursales", map[string]any{}, "GET", "/api/branches"},
		{"usuarios_sucursal", map[string]any{"branchId": 2}, "GET", "/api/users?branchId=2"},
		{"consultar_stock", map[string]any{"branchId": 1}, "GET", "/api/branches/1/stock"},
		{"consultar_stock", map[string]any{"branchId": 1, "productId": 3}, "GET", "/api/branches/1/products/3/stock"},
		{"stock_bajo", map[string]any{"branchId": 1}, "GET", "/api/branches/1/stock?lowStock=true"},
		{"estado_transferencia", map[string]any{"transferId": "abc/1"}, "GET", "/api/transfers/abc%2F1"},
		{"crear_transferencia", map[string]any{"productId": 1, "quantity": 4, "originBranchId": 1, "destinationBranchId": 2}, "POST", "/api/transfers"},
	}
	for _, c := range cases {
		t.Run(c.tool+" "+c.path, func(t *testing.T) {
			gateway := &fakeGateway{status: 200, body: `{"ok":true}`}
			session := connect(t, gateway, &fakeLogs{}, token)

			result := callTool(t, session, c.tool, c.args)

			if result.IsError || text(result) != `{"ok":true}` {
				t.Errorf("Se esperaba la respuesta del gateway, pero vino: %+v", result.Content[0])
			}
			if len(gateway.calls) != 1 {
				t.Fatalf("Se esperaba una llamada al gateway, pero hubo %d", len(gateway.calls))
			}
			got := gateway.calls[0]
			if got.method != c.method || got.path != c.path {
				t.Errorf("Se esperaba %s %s, pero se llamó %s %s", c.method, c.path, got.method, got.path)
			}
			if got.authorization != token {
				t.Errorf("Se esperaba el token del usuario, pero llegó %q", got.authorization)
			}
		})
	}
}

func TestCreateTransfer_SendsTheData(t *testing.T) {
	gateway := &fakeGateway{status: 201, body: `{"id":"t-1"}`}
	session := connect(t, gateway, &fakeLogs{}, token)

	callTool(t, session, "crear_transferencia", map[string]any{"productId": 1, "quantity": 4, "originBranchId": 1, "destinationBranchId": 2})

	want := transferInput{ProductID: 1, Quantity: 4, OriginBranchID: 1, DestinationBranchID: 2}
	if gateway.calls[0].body != want {
		t.Errorf("Se esperaba %+v, pero se mandó %+v", want, gateway.calls[0].body)
	}
}

// CA-33: sin token no se llama a nada, porque el asistente no tiene permisos propios.
func TestTools_WithoutTokenDoNotCall(t *testing.T) {
	gateway := &fakeGateway{status: 200, body: `{}`}
	session := connect(t, gateway, &fakeLogs{}, "")

	result := callTool(t, session, "listar_sucursales", map[string]any{})

	if !result.IsError || len(gateway.calls) != 0 {
		t.Errorf("Se esperaba un error sin llamar al gateway, pero hubo %d llamadas", len(gateway.calls))
	}
}

// CA-33: si el sistema rechaza el pedido, el modelo recibe su mensaje y se lo puede decir al usuario.
func TestTools_PassTheSystemError(t *testing.T) {
	gateway := &fakeGateway{status: 403, body: `{"code":"FORBIDDEN","message":"No podés operar esa sucursal."}`}
	session := connect(t, gateway, &fakeLogs{}, token)

	result := callTool(t, session, "crear_transferencia", map[string]any{"productId": 1, "quantity": 4, "originBranchId": 1, "destinationBranchId": 2})

	if !result.IsError || text(result) != "No podés operar esa sucursal." {
		t.Errorf("Se esperaba el mensaje del sistema, pero vino: %s", text(result))
	}
}

func TestTools_ErrorWithoutMessage(t *testing.T) {
	gateway := &fakeGateway{status: 500, body: `<html>`}
	session := connect(t, gateway, &fakeLogs{}, token)

	result := callTool(t, session, "listar_sucursales", map[string]any{})

	if !result.IsError || !strings.Contains(text(result), "500") {
		t.Errorf("Se esperaba un error con el código, pero vino: %s", text(result))
	}
}

// Si crear no tuvo respuesta, puede haberse creado: no hay que decir que falló.
func TestCreateTransfer_NoResponse(t *testing.T) {
	gateway := &fakeGateway{err: errors.New("timeout")}
	session := connect(t, gateway, &fakeLogs{}, token)

	result := callTool(t, session, "crear_transferencia", map[string]any{"productId": 1, "quantity": 4, "originBranchId": 1, "destinationBranchId": 2})

	if !result.IsError || text(result) != MaybeCreated {
		t.Errorf("Se esperaba el aviso de revisar la lista, pero vino: %s", text(result))
	}
}

func TestQuery_NoResponse(t *testing.T) {
	gateway := &fakeGateway{err: errors.New("timeout")}
	session := connect(t, gateway, &fakeLogs{}, token)

	result := callTool(t, session, "listar_sucursales", map[string]any{})

	if !result.IsError || text(result) == MaybeCreated {
		t.Errorf("Una consulta sin respuesta no tiene nada que revisar, pero vino: %s", text(result))
	}
}

// Faltan datos obligatorios: el SDK lo rechaza sin llamar al gateway.
func TestCreateTransfer_MissingData(t *testing.T) {
	gateway := &fakeGateway{status: 201, body: `{}`}
	session := connect(t, gateway, &fakeLogs{}, token)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "crear_transferencia", Arguments: map[string]any{"productId": 1}})

	if err == nil && !result.IsError {
		t.Errorf("Se esperaba un error por los datos que faltan")
	}
	if len(gateway.calls) != 0 {
		t.Errorf("No se tenía que llamar al gateway, pero hubo %d llamadas", len(gateway.calls))
	}
}

// El token nunca queda en los logs.
func TestLogs_DoNotHaveTheToken(t *testing.T) {
	logs := &fakeLogs{}
	gateway := &fakeGateway{status: 200, body: `{}`}
	session := connect(t, gateway, logs, token)

	callTool(t, session, "listar_sucursales", map[string]any{})

	if len(logs.messages) != 1 {
		t.Fatalf("Se esperaba un log por llamada, pero hubo %d", len(logs.messages))
	}
	if strings.Contains(logs.messages[0], "token-del-usuario") {
		t.Errorf("El token quedó en el log: %s", logs.messages[0])
	}
}
