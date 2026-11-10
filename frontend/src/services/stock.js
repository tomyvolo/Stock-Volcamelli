import httpClient from "./http";

export async function getBranchStock(branchId, lowStock = false) {
  const params = lowStock ? { lowStock: true } : {};
  const res = await httpClient.get(`/branches/${branchId}/stock`, { params });
  return res.data.items || [];
}

export async function getProductStock(branchId, productId) {
  const res = await httpClient.get(`/branches/${branchId}/products/${productId}/stock`);
  return res.data;
}

export async function getMovements(branchId, productId, page = 1, size = 20) {
  const res = await httpClient.get(`/branches/${branchId}/products/${productId}/movements`, { params: { page, size } });
  return res.data;
}

// type es ENTRADA o SALIDA.
export async function adjustStock(branchId, productId, { type, quantity, reason }) {
  const res = await httpClient.post(`/branches/${branchId}/products/${productId}/adjustments`, { type, quantity, reason });
  return res.data;
}
