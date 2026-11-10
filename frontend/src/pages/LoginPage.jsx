import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import PasswordInput from "../components/PasswordInput";
import { useAuth } from "../context/AuthContext";
import { errorMessage } from "../services/http";

function LoginPage() {
  const navigate = useNavigate();
  const { login, loading, isAuthenticated } = useAuth();
  const [form, setForm] = useState({ email: "", password: "" });
  const [error, setError] = useState(null);

  if (isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  function handleChange(e) {
    setForm({ ...form, [e.target.name]: e.target.value });
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError(null);
    try {
      await login(form.email, form.password);
      navigate("/");
    } catch (err) {
      setError(errorMessage(err, "No se pudo iniciar sesión."));
    }
  }

  return (
    <div className="form-shell">
      <h1>Iniciar sesión</h1>
      <form className="auth-form" onSubmit={handleSubmit}>
        <label className="form-field">
          <span>Email</span>
          <input type="email" name="email" value={form.email} onChange={handleChange} required autoComplete="email" />
        </label>

        <label className="form-field">
          <span>Contraseña</span>
          <PasswordInput
            name="password"
            value={form.password}
            onChange={handleChange}
            required
            autoComplete="current-password"
          />
        </label>

        {error && <p className="state-msg error">{error}</p>}

        <button type="submit" className="btn-primary" disabled={loading}>
          {loading ? "Ingresando…" : "Ingresar"}
        </button>
      </form>
    </div>
  );
}

export default LoginPage;
