import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthContext } from "../context/AuthContext";

export function renderWithProviders(ui, { route = "/", auth = null } = {}) {
  const defaultAuth = {
    user: null,
    isAuthenticated: false,
    loading: false,
    login: vi.fn(),
    logout: vi.fn(),
    ...(auth || {}),
  };

  return render(
    <MemoryRouter initialEntries={[route]}>
      <AuthContext.Provider value={defaultAuth}>{ui}</AuthContext.Provider>
    </MemoryRouter>
  );
}

export function authedUser(role, branchId = 1) {
  return {
    user: { id: 1, email: "tomy@volcamelli.com", role, branchId: role === "DIRECTOR" ? null : branchId },
    isAuthenticated: true,
    loading: false,
    login: vi.fn(),
    logout: vi.fn(),
  };
}
