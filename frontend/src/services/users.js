import httpClient from "./http";

export async function getUsers(branchId = "") {
  const params = branchId ? { branchId } : {};
  const res = await httpClient.get("/users", { params });
  return res.data.items || [];
}

export async function createUser(data) {
  const res = await httpClient.post("/create-users", data);
  return res.data;
}

// Solo el director: cambia el rol o la sucursal.
export async function updateUser(id, { role, branchId }) {
  const res = await httpClient.put(`/users/${id}`, { role, branchId });
  return res.data;
}

export async function deleteUser(id) {
  await httpClient.delete(`/users/${id}`);
}
