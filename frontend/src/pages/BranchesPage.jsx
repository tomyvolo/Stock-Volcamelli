import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { getBranches } from "../services/branches";
import { errorMessage } from "../services/http";
import { isDirector } from "../utils/roles";

function BranchesPage() {
  const { user } = useAuth();
  const [branches, setBranches] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    getBranches()
      .then(setBranches)
      .catch((err) => setError(errorMessage(err, "No se pudieron cargar las sucursales.")))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <h1>Sucursales</h1>
        {isDirector(user) && (
          <Link to="/sucursales/nueva" className="btn-primary">
            + Nueva sucursal
          </Link>
        )}
      </div>

      {loading && <p className="state-msg">Cargando…</p>}
      {error && <p className="state-msg error">{error}</p>}

      {!loading && !error && (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Nombre</th>
              <th>Dirección</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {branches.map((b) => (
              <tr key={b.id}>
                <td>{b.name}</td>
                <td>{b.address}</td>
                <td className="admin-row-actions">
                  <Link to={`/usuarios?sucursal=${b.id}`} className="link-action">
                    Usuarios
                  </Link>
                  {isDirector(user) && (
                    <Link to={`/sucursales/${b.id}/editar`} className="link-action">
                      Editar
                    </Link>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

export default BranchesPage;
