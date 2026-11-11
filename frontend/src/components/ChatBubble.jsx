import { useEffect, useRef, useState } from "react";
import { confirmTransfer, sendMessage } from "../services/assistant";
import { errorMessage } from "../services/http";

// El Asistente no guarda nada entre pedidos: le mandamos los últimos 20 mensajes cada vez (ADR-018).
const MAX_HISTORY = 20;
const WELCOME = "Hola, soy el asistente. Preguntame por el stock, los productos o una transferencia, o pedime que cree una.";

function ChatBubble() {
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState([]);
  const [card, setCard] = useState(null);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const bottomRef = useRef(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView?.({ behavior: "smooth" });
  }, [messages, card, open]);

  function addAssistant(text) {
    setMessages((prev) => [...prev, { role: "assistant", content: text }]);
  }

  async function handleSend(e) {
    e.preventDefault();
    const text = input.trim();
    if (!text || sending) return;
    const history = [...messages, { role: "user", content: text }];
    setMessages(history);
    setInput("");
    setCard(null);
    setSending(true);
    try {
      const reply = await sendMessage(history.slice(-MAX_HISTORY));
      addAssistant(reply.message);
      if (reply.confirmation) setCard(reply.confirmation);
    } catch (err) {
      addAssistant(errorMessage(err, "El asistente no está disponible en este momento. Probá de nuevo en unos minutos."));
    } finally {
      setSending(false);
    }
  }

  // La transferencia se crea recién cuando el usuario confirma la tarjeta.
  async function handleConfirm() {
    const transfer = card;
    setCard(null);
    setSending(true);
    try {
      const reply = await confirmTransfer(transfer);
      addAssistant(reply.message);
    } catch (err) {
      addAssistant(errorMessage(err, "No se pudo crear la transferencia. Revisá la lista de transferencias antes de volver a intentarlo."));
    } finally {
      setSending(false);
    }
  }

  function handleDiscard() {
    setCard(null);
    addAssistant("Listo, no creé la transferencia.");
  }

  return (
    <>
      {open && (
        <div className="chat-panel" role="dialog" aria-label="Asistente">
          <div className="chat-header">Asistente</div>
          <div className="chat-messages">
            <div className="chat-msg assistant">{WELCOME}</div>
            {messages.map((m, i) => (
              <div key={i} className={`chat-msg ${m.role}`}>
                {m.content}
              </div>
            ))}
            {card && (
              <div className="chat-card">
                <dl>
                  <dt>Producto</dt>
                  <dd>{card.productName}</dd>
                  <dt>Cantidad</dt>
                  <dd>{card.quantity}</dd>
                  <dt>Desde</dt>
                  <dd>{card.originBranchName}</dd>
                  <dt>Hacia</dt>
                  <dd>{card.destinationBranchName}</dd>
                </dl>
                <div className="modal-actions">
                  <button type="button" className="btn-secondary" onClick={handleDiscard}>
                    Cancelar
                  </button>
                  <button type="button" className="btn-primary" onClick={handleConfirm}>
                    Confirmar
                  </button>
                </div>
              </div>
            )}
            {sending && <div className="chat-msg assistant">Pensando…</div>}
            <div ref={bottomRef} />
          </div>
          <form className="chat-form" onSubmit={handleSend}>
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="Escribí tu pregunta"
              maxLength={2000}
              disabled={sending}
            />
            <button type="submit" className="btn-primary" disabled={sending || !input.trim()}>
              Enviar
            </button>
          </form>
        </div>
      )}
      <button
        type="button"
        className="chat-toggle"
        onClick={() => setOpen(!open)}
        aria-label={open ? "Cerrar el asistente" : "Abrir el asistente"}
      >
        {open ? "×" : "💬"}
      </button>
    </>
  );
}

export default ChatBubble;
