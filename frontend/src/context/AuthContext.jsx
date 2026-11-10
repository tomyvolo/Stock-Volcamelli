import { createContext, useContext, useState } from "react";
import * as authService from "../services/auth";

export const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(() => (authService.getStoredToken() ? authService.getStoredUser() : null));
  const [loading, setLoading] = useState(false);

  async function login(email, password) {
    setLoading(true);
    try {
      const loggedUser = await authService.login(email, password);
      setUser(loggedUser);
      return loggedUser;
    } finally {
      setLoading(false);
    }
  }

  function logout() {
    authService.logout();
    setUser(null);
  }

  const value = {
    user,
    isAuthenticated: !!user,
    loading,
    login,
    logout,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth debe usarse dentro de un AuthProvider");
  }
  return ctx;
}
