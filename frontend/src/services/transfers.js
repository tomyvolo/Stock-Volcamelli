import httpClient from "./http";

export async function getTransfers({ branchId = "", status = "", page = 1, size = 20 } = {}) {
  const params = { page, size };
  if (branchId) params.branchId = branchId;
  if (status) params.status = status;
  const res = await httpClient.get("/transfers", { params });
  return res.data;
}

export async function getTransfer(id) {
  const res = await httpClient.get(`/transfers/${id}`);
  return res.data;
}

// desiredArrival y notes son opcionales. La reserva en Rumbo sale en segundo plano.
export async function createTransfer(data) {
  const res = await httpClient.post("/transfers", data);
  return res.data;
}

export async function cancelTransfer(id, reason) {
  const res = await httpClient.post(`/transfers/${id}/cancel`, { reason });
  return res.data;
}
