import httpClient from "./http";

export async function getProducts(page = 1, size = 20) {
  const res = await httpClient.get("/products", { params: { page, size } });
  return res.data;
}

export async function getProduct(id) {
  const res = await httpClient.get(`/products/${id}`);
  return res.data;
}

// La búsqueda pide texto; el proveedor es opcional. El orden es por relevancia o por nombre.
export async function searchProducts({ q, supplier = "", sort = "relevancia", page = 1, size = 20 }) {
  const params = { q, sort, page, size };
  if (supplier) params.supplier = supplier;
  const res = await httpClient.get("/products/search", { params });
  return res.data;
}

export async function createProduct(data) {
  const res = await httpClient.post("/products", data);
  return res.data;
}

export async function updateProduct(id, data) {
  const res = await httpClient.put(`/products/${id}`, data);
  return res.data;
}
