export function formatDateTime(dateStr) {
  if (!dateStr) return "—";
  return new Date(dateStr).toLocaleString("es-AR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatTime(dateStr) {
  if (!dateStr) return "—";
  return new Date(dateStr).toLocaleTimeString("es-AR", { hour: "2-digit", minute: "2-digit" });
}

// formatSlot muestra la franja de Rumbo, por ejemplo "20/11 de 10:00 a 12:00".
export function formatSlot(start, end) {
  if (!start || !end) return "—";
  const day = new Date(start).toLocaleDateString("es-AR", { day: "2-digit", month: "2-digit" });
  return `${day} de ${formatTime(start)} a ${formatTime(end)}`;
}
