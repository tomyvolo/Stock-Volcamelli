import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import BranchSelect from "../components/BranchSelect";
import PasswordInput from "../components/PasswordInput";
import { useAuth } from "../context/AuthContext";
import { getBranches } from "../services/branches";
import { errorMessage } from "../services/http";
import { createUser, updateUser } from "../services/users";
import { ALL_ROLES, DIRECTOR, ROLE_LABEL, VENDEDOR, isDirector } from "../utils/roles";

// Alta de usuarios (director: cualquier rol; admin: vendedores de su sucursal) y edición del rol y la
// sucursal (solo el director). Los cambios se aplican cuando el usuario vuelve a iniciar sesión.
function UserFormPage() {
  const { id } = useParams();
  const { state: editingUser } = useLocation();
  const { user } = useAuth();
  const navigate = useNavigate();
  const editing = Boolean(id);
  const director = isDirector(user);
  const [branches, setBranches] = useState([]);
  const [form, setForm] = useState({
    name: editingUser?.name || "",
    email: editingUser?.email || "",
    password: "",
    role: editingUser?.role || VENDEDOR,
    branchId: String(editingUser?.branchId || (director ? "" : user.branchId)),
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    getBranches().then(setBranches).catch(() => setError("No se pudieron cargar las sucursales."));
  }, []);

  function handleChange(e) {
    setForm({ ...form, [e.target.name]: e.target.value });
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    // El director no tiene sucursal; los demás roles tienen una sola.
    const branchId = form.role === DIRECTOR ? null : Number(form.branchId);
    try {
      if (editing) {
        await updateUser(id, { role: form.role, branchId });
      } else {
        await createUser({ name: form.name.trim(), email: form.email.trim(), password: form.password, role: form.role, branchId });
      }
      navigate("/usuarios");
    } catch (err) {
      setError(errorMessage(err, "No se pudo guardar el usuario."));
      setSaving(false);
    }
  }

  if (editing && !editingUser) {
    return (
      <p className="state-msg">
        Abrí la edición desde la <Link to="/usuarios">lista de usuarios</Link>.
      </p>
    );
  }

  // El admin solo da de alta vendedores de su propia sucursal.
  const roles = director ? ALL_ROLES : [VENDEDOR];
  const branchOptions = director ? branches : branches.filter((b) => b.id === user.branchId);

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <div>
          <Link to="/usuarios" className="btn-back">
            ← Usuarios
          </Link>
          <h1>{editing ? `Editar a ${editingUser.name}` : "Nuevo usuario"}</h1>
        </div>
      </div>

      <form className="admin-form" onSubmit={handleSubmit}>
        {!editing && (
          <>
            <label className="form-field">
              <span>Nombre</span>
              <input name="name" value={form.name} onChange={handleChange} required />
            </label>
            <label className="form-field">
              <span>Email</span>
              <input type="email" name="email" value={form.email} onChange={handleChange} required />
            </label>
            <label className="form-field">
              <span>Contraseña (6 caracteres o más)</span>
              <PasswordInput name="password" value={form.password} onChange={handleChange} minLength={6} required autoComplete="new-password" />
            </label>
          </>
        )}
        <div className="form-row">
          <label className="form-field">
            <span>Rol</span>
            <select name="role" value={form.role} onChange={handleChange}>
              {roles.map((r) => (
                <option key={r} value={r}>
                  {ROLE_LABEL[r]}
                </option>
              ))}
            </select>
          </label>
          {form.role !== DIRECTOR && (
            <label className="form-field">
              <span>Sucursal</span>
              <BranchSelect
                branches={branchOptions}
                name="branchId"
                value={form.branchId}
                onChange={handleChange}
                emptyLabel={director ? "Elegí la sucursal" : undefined}
                required
              />
            </label>
          )}
        </div>
        {editing && (
          <p className="report-subtitle">El cambio se aplica cuando el usuario vuelve a iniciar sesión.</p>
        )}

        {error && <p className="state-msg error">{error}</p>}

        <div className="form-actions">
          <button type="submit" className="btn-primary" disabled={saving}>
            {saving ? "Guardando…" : "Guardar"}
          </button>
        </div>
      </form>
    </div>
  );
}

export default UserFormPage;
