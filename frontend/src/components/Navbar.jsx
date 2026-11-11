import { Link, NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { useRealtime } from "../context/RealtimeContext";
import { ROLE_LABEL } from "../utils/roles";

const LINKS = [
  { to: "/", label: "Stock", end: true },
  { to: "/productos", label: "Productos" },
  { to: "/transferencias", label: "Transferencias" },
  { to: "/sucursales", label: "Sucursales" },
  { to: "/usuarios", label: "Usuarios" },
];

function Navbar() {
  const { isAuthenticated, user, logout } = useAuth();
  const navigate = useNavigate();
  const connected = useRealtime(() => {});

  function handleLogout() {
    if (!confirm("¿Cerrar sesión?")) return;
    logout();
    navigate("/login");
  }

  return (
    <header className="navbar">
      <div className="navbar-inner">
        <Link to="/" className="navbar-brand">
          📦 Stock Volcamelli
        </Link>

        <nav className="navbar-links">
          {isAuthenticated &&
            LINKS.map((link) => (
              <NavLink
                key={link.to}
                to={link.to}
                end={link.end}
                className={({ isActive }) => "navbar-link" + (isActive ? " active" : "")}
              >
                {link.label}
              </NavLink>
            ))}
        </nav>

        <div className="navbar-actions">
          {isAuthenticated ? (
            <>
              <span className={connected ? "live-badge" : "live-badge off"}>
                {connected ? "● En vivo" : "○ Sin conexión en vivo"}
              </span>
              <span className="navbar-user">
                {user.email} · {ROLE_LABEL[user.role] || user.role}
                {user.branchId ? ` · Sucursal ${user.branchId}` : ""}
              </span>
              <button type="button" className="btn-secondary" onClick={handleLogout}>
                Cerrar sesión
              </button>
            </>
          ) : (
            <Link to="/login" className="btn-primary">
              Iniciar sesión
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}

export default Navbar;
