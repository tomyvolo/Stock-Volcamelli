import { readToken } from "../auth";

// Arma un JWT de prueba con los mismos datos que firma Usuarios. La firma no importa: el frontend no la valida.
function token(claims) {
  const base64url = (obj) =>
    btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(obj))))
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/, "");
  return `${base64url({ alg: "HS256", typ: "JWT" })}.${base64url(claims)}.firma`;
}

describe("readToken", () => {
  it("saca el id, el email, el rol y la sucursal del token", () => {
    const user = readToken(token({ user_id: 7, email: "vendedor@volcamelli.com", role: "VENDEDOR", branch_id: 2 }));

    expect(user).toEqual({ id: 7, email: "vendedor@volcamelli.com", role: "VENDEDOR", branchId: 2 });
  });

  it("el director no tiene sucursal", () => {
    const user = readToken(token({ user_id: 1, email: "director@volcamelli.com", role: "DIRECTOR", branch_id: null }));

    expect(user.branchId).toBeNull();
  });

  it("lee bien los acentos", () => {
    const user = readToken(token({ user_id: 3, email: "josé@volcamelli.com", role: "VENDEDOR", branch_id: 1 }));

    expect(user.email).toBe("josé@volcamelli.com");
  });
});
