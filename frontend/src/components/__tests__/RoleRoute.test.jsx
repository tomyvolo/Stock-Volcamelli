import { screen } from "@testing-library/react";
import { Route, Routes } from "react-router-dom";
import RoleRoute from "../RoleRoute";
import { authedUser, renderWithProviders } from "../../test/utils";

function renderRoute(auth) {
  return renderWithProviders(
    <Routes>
      <Route path="/" element={<p>Stock</p>} />
      <Route path="/login" element={<p>Login</p>} />
      <Route
        path="/productos/nuevo"
        element={
          <RoleRoute roles={["DIRECTOR"]}>
            <p>Nuevo producto</p>
          </RoleRoute>
        }
      />
    </Routes>,
    { route: "/productos/nuevo", auth }
  );
}

describe("RoleRoute", () => {
  it("deja entrar al rol permitido", () => {
    renderRoute(authedUser("DIRECTOR"));
    expect(screen.getByText("Nuevo producto")).toBeInTheDocument();
  });

  it("manda al inicio a un rol que no tiene permiso", () => {
    renderRoute(authedUser("VENDEDOR"));
    expect(screen.getByText("Stock")).toBeInTheDocument();
  });

  it("manda al login si no hay sesión", () => {
    renderRoute(null);
    expect(screen.getByText("Login")).toBeInTheDocument();
  });
});
