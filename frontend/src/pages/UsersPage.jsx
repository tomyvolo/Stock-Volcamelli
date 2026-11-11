import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import BranchSelect from "../components/BranchSelect";
import { useAuth } from "../context/AuthContext";
import { getBranches } from "../services/branches";
import { errorMessage } from "../services/http";
import { deleteUser, getUsers } from "../services/users";
import { ROLE_LABEL, canCreateUsers, isDirector } from "../utils/roles";

// Usuarios de todas las sucursales o de una. El director y el admin crean; solo el director edita y borra.
function UsersPage() {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const branchId = params.get("sucursal") || "";
  const [branches, setBranches] = useState([]);
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [msg, setMsg] = useState(null);

  useEffect(() => {
    getBranches().then(setBranches).catch(() => {});
  }, []);

  const load = useCallback(async () => {
    setError(null);
    try {
      setUsers(await getUsers(branchId));
    } catch (err) {
      setError(errorMessage(err, "No se pudieron cargar los usuarios."));
    } finally {
      setLoading(false);
    }
  }, [branchId]);

  useEffect(() => {
    setLoading(true);
    load();
  }, [load]);

  async function handleDelete(u) {
    if (!confirm(`¿Borrar a ${u.name}?`)) return;
    setMsg(null);
    try {
      await deleteUser(u.id);
      setMsg({ type: "success", text: "Usuario borrado." });
      await load();
    } catch (err) {
      setMsg({ type: "error", text: errorMessage(err, "No se pudo borrar el usuario.") });
    }
  }

  const branchName = (id) => branches.find((b) => b.id === id)?.name || (id ? `Sucursal ${id}` : "—");

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <h1>Usuarios</h1>
        {canCreateUsers(user) && (
          <Link to="/usuarios/nuevo" className="btn-primary">
            + Nuevo usuario
          </Link>
        )}
      </div>

      <div className="catalog-filters">
        <BranchSelect
          branches={branches}
          value={branchId}
          onChange={(e) => setParams(e.target.value ? { sucursal: e.target.value } : {})}
          emptyLabel="Todas las sucursales"
        />
      </div>

      {msg && <p className={`state-msg ${msg.type}`}>{msg.text}</p>}
      {loading && <p className="state-msg">Cargando…</p>}
      {error && <p className="state-msg error">{error}</p>}
      {!loading && !error && users.length === 0 && <p className="state-msg">No hay usuarios.</p>}

      {!loading && !error && users.length > 0 && (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Nombre</th>
              <th>Email</th>
              <th>Rol</th>
              <th>Sucursal</th>
              {isDirector(user) && <th></th>}
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.name}</td>
                <td>{u.email}</td>
                <td>{ROLE_LABEL[u.role] || u.role}</td>
                <td>{branchName(u.branchId)}</td>
                {isDirector(user) && (
                  <td className="admin-row-actions">
                    <Link to={`/usuarios/${u.id}/editar`} state={u} className="link-action">
                      Editar
                    </Link>
                    {u.id !== user.id && (
                      <button type="button" className="link-action danger" onClick={() => handleDelete(u)}>
                        Borrar
                      </button>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

export default UsersPage;
