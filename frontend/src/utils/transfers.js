// Estados de una transferencia (SPEC.md) y cómo se muestran.
export const STATUS_LABEL = {
  SIN_STOCK: "Sin stock",
  FALLIDA: "Fallida",
  RESERVADA: "Reservada",
  PROGRAMADA: "Programada",
  EN_CURSO: "En curso",
  COMPLETADA: "Completada",
  RECHAZADA: "Rechazada",
  CANCELADA: "Cancelada",
  NO_REALIZADA: "No realizada",
};

export const STATUSES = Object.keys(STATUS_LABEL);

export function statusClass(status) {
  if (status === "COMPLETADA") return "tag-active";
  if (status === "RESERVADA") return "tag-pending";
  if (status === "PROGRAMADA" || status === "EN_CURSO") return "tag-progress";
  return "tag-cancelled";
}

// Se puede cancelar mientras el viaje no empezó y no se pidió ya la cancelación.
export function canCancel(transfer) {
  return (transfer.status === "RESERVADA" || transfer.status === "PROGRAMADA") && !transfer.cancellationRequested;
}
