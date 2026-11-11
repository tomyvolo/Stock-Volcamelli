// Package chat tiene el ciclo del asistente: le manda la conversación al modelo, ejecuta las herramientas
// que pide y le devuelve los resultados, hasta que responde con texto (ADR-018).
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"asistente/model"
	"asistente/tools"
)

const (
	// MaxRounds es cuántas veces como máximo el modelo puede pedir herramientas para una pregunta.
	MaxRounds = 5
	// MaxHistory es cuántos mensajes del historial le mandamos al modelo.
	MaxHistory = 20
	// MaxLength es el largo máximo de cada mensaje del historial, para no mandarle al modelo textos enormes.
	MaxLength = 2000

	createTransfer = "crear_transferencia"
)

// Respuestas fijas del chat.
const (
	NotSolved    = "No pude resolverlo. Probá preguntarlo de otra forma."
	ConfirmAsk   = "¿Confirmás esta transferencia?"
	MaybeCreated = "No hubo respuesta al crear la transferencia y puede haberse creado igual. Revisá la lista de transferencias antes de volver a intentarlo."
)

// ErrUnavailable es que el modelo o el servidor MCP no respondieron: el chat dice que el asistente no está disponible.
var ErrUnavailable = errors.New("el asistente no está disponible")

// systemPrompt le explica al modelo qué es y qué reglas sigue.
const systemPrompt = `Sos el asistente de Stock Volcamelli, un sistema de stock de una cadena de sucursales.
Respondés en castellano, corto y claro.
Para cualquier dato del sistema usás las herramientas: nunca inventes productos, sucursales, cantidades ni ids.
Los ids los sacás de las herramientas (buscar_producto, listar_sucursales), nunca se los pidas al usuario.
Para crear una transferencia necesitás el producto, la cantidad, el origen y el destino. Si falta algo, preguntalo.
Cuando pidas crear_transferencia, el sistema le muestra al usuario una tarjeta para que la confirme: no digas que ya está creada.
Si una herramienta responde con un error, explicáselo al usuario con tus palabras.
Solo podés consultar productos, stock, sucursales y transferencias, y crear transferencias.
Si te piden otra cosa (un ajuste de stock, crear usuarios, sucursales o productos), respondé que no podés hacerlo desde el chat y que se hace desde la pantalla correspondiente, sin pedir más datos.`

// Model es el modelo de lenguaje. Lo implementa el paquete model, y en los tests una versión falsa.
type Model interface {
	Complete(ctx context.Context, messages []model.Message, tools []model.Tool) (model.Message, error)
}

// Tools abre una conexión al servidor MCP con el token del usuario. Lo implementa el paquete tools.
type Tools interface {
	Open(ctx context.Context, authorization, trackID string) (tools.Session, error)
}

// Logger escribe los logs. nivel es info, warn o error.
type Logger interface {
	Write(trackID, stage, level, message string)
}

const stage = "asistente"

// Service es el asistente.
type Service struct {
	Model Model
	Tools Tools
	Logs  Logger
}

// Message es un mensaje del historial que guarda el frontend.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Transfer es lo que el usuario confirma en la tarjeta: los ids para crearla y los nombres para que sepa qué confirma.
type Transfer struct {
	ProductID             uint   `json:"productId"`
	ProductName           string `json:"productName,omitempty"`
	Quantity              int    `json:"quantity"`
	OriginBranchID        uint   `json:"originBranchId"`
	OriginBranchName      string `json:"originBranchName,omitempty"`
	DestinationBranchID   uint   `json:"destinationBranchId"`
	DestinationBranchName string `json:"destinationBranchName,omitempty"`
}

// Reply es lo que ve el usuario: un texto y, si el asistente quiere crear una transferencia, la tarjeta.
type Reply struct {
	Message      string    `json:"message"`
	Confirmation *Transfer `json:"confirmation,omitempty"`
}

// Chat responde el último mensaje del historial.
func (s Service) Chat(ctx context.Context, authorization, trackID string, history []Message) (Reply, error) {
	session, err := s.Tools.Open(ctx, authorization, trackID)
	if err != nil {
		return Reply{}, s.unavailable(trackID, err)
	}
	defer session.Close()
	toolList, err := session.List(ctx)
	if err != nil {
		return Reply{}, s.unavailable(trackID, err)
	}

	messages := conversation(history)
	for round := 0; round < MaxRounds; round++ {
		answer, err := s.Model.Complete(ctx, messages, toolList)
		if err != nil {
			return Reply{}, s.unavailable(trackID, err)
		}
		if len(answer.ToolCalls) == 0 {
			return Reply{Message: answer.Content}, nil
		}

		messages = append(messages, answer)
		for _, call := range answer.ToolCalls {
			name := call.Function.Name
			// crear_transferencia nunca se ejecuta acá: se le muestra la tarjeta al usuario y se crea
			// recién si la confirma, con Confirm.
			if name == createTransfer {
				return s.confirmation(ctx, session, trackID, call.Function.Arguments), nil
			}
			text, isError, err := session.Call(ctx, name, json.RawMessage(call.Function.Arguments))
			if err != nil {
				s.Logs.Write(trackID, stage, "warn", fmt.Sprintf("la herramienta %s falló: %v", name, err))
				text, isError = "La herramienta no respondió.", true
			}
			if isError {
				text = "Error: " + text
			}
			messages = append(messages, model.Message{Role: "tool", ToolCallID: call.ID, Content: text})
		}
	}
	s.Logs.Write(trackID, stage, "warn", fmt.Sprintf("el modelo no terminó en %d vueltas", MaxRounds))
	return Reply{Message: NotSolved}, nil
}

// Confirm crea la transferencia que el usuario confirmó en la tarjeta.
func (s Service) Confirm(ctx context.Context, authorization, trackID string, transfer Transfer) (Reply, error) {
	session, err := s.Tools.Open(ctx, authorization, trackID)
	if err != nil {
		return Reply{}, s.unavailable(trackID, err)
	}
	defer session.Close()

	arguments, _ := json.Marshal(map[string]any{
		"productId": transfer.ProductID, "quantity": transfer.Quantity,
		"originBranchId": transfer.OriginBranchID, "destinationBranchId": transfer.DestinationBranchID,
	})
	text, isError, err := session.Call(ctx, createTransfer, arguments)
	if err != nil {
		// No sabemos si llegó a crearse: no decimos que falló.
		s.Logs.Write(trackID, stage, "error", "crear la transferencia confirmada no tuvo respuesta: "+err.Error())
		return Reply{Message: MaybeCreated}, nil
	}
	if isError {
		return Reply{Message: text}, nil
	}

	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &created); err != nil || created.ID == "" {
		return Reply{Message: "Listo, la transferencia se creó."}, nil
	}
	s.Logs.Write(trackID, stage, "info", fmt.Sprintf("transferencia %s creada desde el chat: %s", created.ID, created.Status))
	if created.Status == "SIN_STOCK" {
		return Reply{Message: fmt.Sprintf("Creé la transferencia %s, pero el origen no tenía stock disponible suficiente y quedó SIN_STOCK.", created.ID)}, nil
	}
	return Reply{Message: fmt.Sprintf("Listo, creé la transferencia %s. Quedó %s y la reserva del traslado en Rumbo sale sola.", created.ID, created.Status)}, nil
}

// confirmation arma la tarjeta. El modelo pide la herramienta con ids, y con un id el usuario no puede
// controlar qué confirma: buscamos los nombres del producto y de las sucursales antes de mostrarla.
func (s Service) confirmation(ctx context.Context, session tools.Session, trackID, arguments string) Reply {
	var transfer Transfer
	if err := json.Unmarshal([]byte(arguments), &transfer); err != nil ||
		transfer.ProductID == 0 || transfer.OriginBranchID == 0 || transfer.DestinationBranchID == 0 || transfer.Quantity <= 0 {
		return Reply{Message: "No me quedaron claros los datos de la transferencia. Decime el producto, la cantidad, el origen y el destino."}
	}

	branches, err := branchNames(ctx, session)
	if err != nil {
		s.Logs.Write(trackID, stage, "warn", "no se pudieron buscar las sucursales para la tarjeta: "+err.Error())
		return Reply{Message: "No pude buscar las sucursales para armar la transferencia. Probá de nuevo en unos minutos."}
	}
	transfer.OriginBranchName = branches[transfer.OriginBranchID]
	transfer.DestinationBranchName = branches[transfer.DestinationBranchID]
	if transfer.OriginBranchName == "" || transfer.DestinationBranchName == "" {
		return Reply{Message: "Alguna de las sucursales de la transferencia no existe. Decime de cuál a cuál querés transferir."}
	}

	transfer.ProductName, err = productName(ctx, session, transfer.OriginBranchID, transfer.ProductID)
	if err != nil {
		return Reply{Message: "No encontré ese producto en la sucursal de origen. Decime cuál querés transferir."}
	}
	return Reply{Message: ConfirmAsk, Confirmation: &transfer}
}

func branchNames(ctx context.Context, session tools.Session) (map[uint]string, error) {
	text, isError, err := session.Call(ctx, "listar_sucursales", json.RawMessage(`{}`))
	if err != nil {
		return nil, err
	}
	if isError {
		return nil, errors.New(text)
	}
	var list struct {
		Items []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(text), &list); err != nil {
		return nil, err
	}
	names := map[uint]string{}
	for _, branch := range list.Items {
		names[branch.ID] = branch.Name
	}
	return names, nil
}

func productName(ctx context.Context, session tools.Session, branchID, productID uint) (string, error) {
	arguments, _ := json.Marshal(map[string]uint{"branchId": branchID, "productId": productID})
	text, isError, err := session.Call(ctx, "consultar_stock", arguments)
	if err != nil {
		return "", err
	}
	if isError {
		return "", errors.New(text)
	}
	var stock struct {
		ProductName string `json:"productName"`
	}
	if err := json.Unmarshal([]byte(text), &stock); err != nil || stock.ProductName == "" {
		return "", errors.New("la consulta de stock no trajo el nombre del producto")
	}
	return stock.ProductName, nil
}

// conversation arma lo que ve el modelo: las reglas y los últimos mensajes del historial. Solo pasan los
// mensajes del usuario y del asistente, recortados: el frontend no puede meterle otras instrucciones.
func conversation(history []Message) []model.Message {
	if len(history) > MaxHistory {
		history = history[len(history)-MaxHistory:]
	}
	messages := []model.Message{{Role: "system", Content: systemPrompt}}
	for _, m := range history {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := []rune(m.Content)
		if len(content) > MaxLength {
			content = content[:MaxLength]
		}
		messages = append(messages, model.Message{Role: m.Role, Content: string(content)})
	}
	return messages
}

func (s Service) unavailable(trackID string, err error) error {
	s.Logs.Write(trackID, stage, "error", err.Error())
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}
