import httpClient, { TOKEN_KEY, USER_KEY } from "./http";

// El login devuelve solo el token. El id, el email, el rol y la sucursal van adentro del token: los leemos
// para saber qué mostrar. No hace falta validarlo acá, eso lo hace el gateway en cada pedido.
export async function login(email, password) {
  const res = await httpClient.post("/auth/login", { email, password });
  const { token } = res.data;
  const user = readToken(token);
  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(USER_KEY, JSON.stringify(user));
  return user;
}

export function readToken(token) {
  // La parte del medio del JWT es base64url: la pasamos a base64 común antes de decodificarla.
  let payload = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
  payload += "=".repeat((4 - (payload.length % 4)) % 4);
  const bytes = Uint8Array.from(atob(payload), (c) => c.charCodeAt(0));
  const claims = JSON.parse(new TextDecoder().decode(bytes));
  return {
    id: claims.user_id,
    email: claims.email,
    role: claims.role,
    branchId: claims.branch_id ?? null,
  };
}

export function logout() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

export function getStoredToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function getStoredUser() {
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}
