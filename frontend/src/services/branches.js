import httpClient from "./http";

export async function getBranches() {
  const res = await httpClient.get("/branches");
  return res.data.items || [];
}

export async function getBranch(id) {
  const res = await httpClient.get(`/branches/${id}`);
  return res.data;
}

export async function createBranch(data) {
  const res = await httpClient.post("/branches", data);
  return res.data;
}

export async function updateBranch(id, data) {
  const res = await httpClient.put(`/branches/${id}`, data);
  return res.data;
}
