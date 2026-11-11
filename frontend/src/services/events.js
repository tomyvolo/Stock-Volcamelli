import { getStoredToken } from "./auth";

// Esperas entre reconexiones: 1, 2, 4... hasta 30 segundos (ADR-019).
const MAX_WAIT = 30000;

// parseEvents separa el texto que llega en mensajes de Server-Sent Events ("event: X\ndata: {...}\n\n").
// Devuelve los mensajes completos y lo que quedó a medias, que se completa con lo próximo que llegue.
// Las líneas que empiezan con ":" son los avisos de que la conexión sigue viva: se ignoran.
export function parseEvents(buffer) {
  const events = [];
  const blocks = buffer.split("\n\n");
  const rest = blocks.pop();
  for (const block of blocks) {
    let type = "message";
    let data = "";
    for (const line of block.split("\n")) {
      if (line.startsWith("event:")) type = line.slice(6).trim();
      else if (line.startsWith("data:")) data += line.slice(5).trim();
    }
    if (!data) continue;
    try {
      events.push({ type, data: JSON.parse(data) });
    } catch {
      // Un mensaje que no es JSON no le sirve a ninguna pantalla.
    }
  }
  return { events, rest };
}

// openEvents abre GET /api/events y llama a onEvent con cada aviso. EventSource no deja mandar el header
// Authorization, así que usamos fetch y leemos la respuesta a medida que llega (ADR-019).
// Si se corta, reconecta esperando cada vez más y llama a onStatus(true) al volver, para que las pantallas
// recarguen lo que se perdió. Devuelve una función para cerrarla.
export function openEvents(onEvent, onStatus) {
  let closed = false;
  let controller = null;
  let wait = 1000;

  async function connect() {
    while (!closed) {
      controller = new AbortController();
      try {
        const res = await fetch("/api/events", {
          headers: { Authorization: `Bearer ${getStoredToken()}` },
          signal: controller.signal,
        });
        if (res.status === 401) {
          // El token venció: el resto de la app ya manda al login.
          onStatus(false);
          return;
        }
        if (!res.ok || !res.body) throw new Error(`respondió ${res.status}`);

        wait = 1000;
        onStatus(true);
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        while (!closed) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const parsed = parseEvents(buffer);
          buffer = parsed.rest;
          parsed.events.forEach(onEvent);
        }
      } catch {
        // Se cortó o no conectó: probamos de nuevo más abajo.
      }
      if (closed) return;
      onStatus(false);
      await new Promise((resolve) => setTimeout(resolve, wait));
      wait = Math.min(wait * 2, MAX_WAIT);
    }
  }

  connect();
  return () => {
    closed = true;
    controller?.abort();
  };
}
