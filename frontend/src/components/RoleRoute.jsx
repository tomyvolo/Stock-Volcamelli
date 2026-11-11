import { Navigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

// RoleRoute deja entrar solo a los roles indicados. Es para no mostrar pantallas que el gateway igual rechazaría.
function RoleRoute({ roles, children }) {
  const { isAuthenticated, user } = useAuth();
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  if (!roles.includes(user.role)) {
    return <Navigate to="/" replace />;
  }
  return children;
}

export default RoleRoute;
