// Package tools define las herramientas del servidor MCP. No tiene nada de IA:
// cada herramienta es una llamada a la API del sistema con el token del usuario.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Gateway es la API del sistema. Lo implementa el paquete gateway, y en los tests una versión falsa.
type Gateway interface {
	Do(ctx context.Context, method, path, authorization string, body any) (int, []byte, error)
}

// Logger escribe los logs de cada herramienta. nivel es info, warn o error.
type Logger interface {
	Write(trackID, stage, level, message string)
}

// MaybeCreated es lo que respondemos si crear una transferencia no tuvo respuesta: puede haberse creado igual.
const MaybeCreated = "No hubo respuesta al crear la transferencia y puede haberse creado igual. Hay que revisar la lista de transferencias antes de volver a intentarlo."

const stage = "mcp"

// Toolbox tiene lo que necesitan las herramientas.
type Toolbox struct {
	Gateway Gateway
	Logs    Logger
	// Count cuenta cada llamada para las métricas. Puede ser nil.
	Count func(tool, result string)
}

type searchInput struct {
	Query string `json:"query" jsonschema:"texto a buscar en el nombre y la descripción del producto"`
}

type noInput struct{}

type branchInput struct {
	BranchID uint `json:"branchId" jsonschema:"id de la sucursal; se obtiene con listar_sucursales"`
}

type stockInput struct {
	BranchID  uint `json:"branchId" jsonschema:"id de la sucursal; se obtiene con listar_sucursales"`
	ProductID uint `json:"productId,omitempty" jsonschema:"id del producto; si no se pasa, devuelve el stock de toda la sucursal"`
}

type transferInput struct {
	ProductID           uint `json:"productId" jsonschema:"id del producto; se obtiene con buscar_producto"`
	Quantity            int  `json:"quantity" jsonschema:"cantidad de unidades a transferir, mayor a 0"`
	OriginBranchID      uint `json:"originBranchId" jsonschema:"id de la sucursal de origen"`
	DestinationBranchID uint `json:"destinationBranchId" jsonschema:"id de la sucursal de destino"`
}

type transferIDInput struct {
	TransferID string `json:"transferId" jsonschema:"id de la transferencia"`
}

// NewServer arma el servidor con las 7 herramientas.
func NewServer(box Toolbox) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "stock-volcamelli", Version: "1.0.0"}, nil)

	add(box, server, "buscar_producto", "Busca productos por texto y devuelve su id y su nombre.",
		func(in searchInput) call {
			return call{method: http.MethodGet, path: "/api/products/search?q=" + url.QueryEscape(in.Query)}
		})
	add(box, server, "listar_sucursales", "Lista las sucursales con su id, nombre y dirección.",
		func(noInput) call { return call{method: http.MethodGet, path: "/api/branches"} })
	add(box, server, "usuarios_sucursal", "Dice quiénes trabajan en una sucursal, con su email y su rol.",
		func(in branchInput) call {
			return call{method: http.MethodGet, path: "/api/users?branchId=" + strconv.FormatUint(uint64(in.BranchID), 10)}
		})
	add(box, server, "consultar_stock", "Devuelve el stock disponible de un producto en una sucursal, o de toda la sucursal si no se indica el producto.",
		func(in stockInput) call {
			path := fmt.Sprintf("/api/branches/%d/stock", in.BranchID)
			if in.ProductID != 0 {
				path = fmt.Sprintf("/api/branches/%d/products/%d/stock", in.BranchID, in.ProductID)
			}
			return call{method: http.MethodGet, path: path}
		})
	add(box, server, "stock_bajo", "Lista los productos con stock bajo en una sucursal.",
		func(in branchInput) call {
			return call{method: http.MethodGet, path: fmt.Sprintf("/api/branches/%d/stock?lowStock=true", in.BranchID)}
		})
	add(box, server, "crear_transferencia", "Crea una transferencia de stock entre dos sucursales. Reserva las unidades en el origen.",
		func(in transferInput) call {
			return call{method: http.MethodPost, path: "/api/transfers", body: in, onNoResponse: MaybeCreated}
		})
	add(box, server, "estado_transferencia", "Devuelve el estado y el detalle de una transferencia.",
		func(in transferIDInput) call {
			return call{method: http.MethodGet, path: "/api/transfers/" + url.PathEscape(in.TransferID)}
		})
	return server
}

// call es el pedido a la API que le corresponde a una herramienta.
type call struct {
	method string
	path   string
	body   any
	// onNoResponse reemplaza el mensaje de error si el gateway no respondió.
	onNoResponse string
}

// add registra una herramienta: cada una solo dice qué pedido hacer, y el resto es igual para todas.
func add[In any](box Toolbox, server *mcp.Server, name, description string, build func(In) call) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, request *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
			return box.run(ctx, request, name, build(input)), nil, nil
		})
}

func (box Toolbox) run(ctx context.Context, request *mcp.CallToolRequest, name string, c call) *mcp.CallToolResult {
	var authorization, trackID string
	if request != nil && request.Extra != nil {
		authorization = request.Extra.Header.Get("Authorization")
		trackID = request.Extra.Header.Get("X-Track-ID")
	}
	start := time.Now()

	// Sin token no hay usuario, y el asistente no tiene permisos propios.
	if authorization == "" {
		box.finish(trackID, name, "error", "warn", "sin token", start)
		return failure("Falta el token del usuario.")
	}

	status, body, err := box.Gateway.Do(ctx, c.method, c.path, authorization, c.body)
	if err != nil {
		box.finish(trackID, name, "error", "error", err.Error(), start)
		if c.onNoResponse != "" {
			return failure(c.onNoResponse)
		}
		return failure("El sistema no respondió. Se puede volver a intentar en unos minutos.")
	}
	if status < 200 || status >= 300 {
		level := "warn"
		if status >= 500 {
			level = "error"
		}
		box.finish(trackID, name, "error", level, fmt.Sprintf("respondió %d", status), start)
		return failure(errorMessage(status, body))
	}

	box.finish(trackID, name, "ok", "info", fmt.Sprintf("respondió %d", status), start)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}
}

func (box Toolbox) finish(trackID, name, result, level, detail string, start time.Time) {
	if box.Count != nil {
		box.Count(name, result)
	}
	box.Logs.Write(trackID, stage, level, fmt.Sprintf("herramienta %s: %s (%d ms)", name, detail, time.Since(start).Milliseconds()))
}

func failure(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}

// errorMessage saca el mensaje del cuerpo de error del sistema ({code, message}), que ya está pensado para el usuario.
func errorMessage(status int, body []byte) string {
	var parsed struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Message == "" {
		return fmt.Sprintf("El sistema respondió con el error %d.", status)
	}
	return parsed.Message
}
