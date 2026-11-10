import httpClient from "./http";

// messages es el historial de la conversación, que guarda el frontend: [{role: "user"|"assistant", content}].
// Responde {message, confirmation}: si el asistente quiere crear una transferencia, confirmation trae la tarjeta.
export async function sendMessage(messages) {
  const res = await httpClient.post("/assistant/chat", { messages }, { timeout: 35000 });
  return res.data;
}

// Crea la transferencia que el usuario confirmó en la tarjeta.
export async function confirmTransfer(transfer) {
  const res = await httpClient.post("/assistant/confirm", transfer, { timeout: 35000 });
  return res.data;
}
