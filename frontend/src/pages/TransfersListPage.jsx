import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import BranchSelect from "../components/BranchSelect";
import Pagination from "../components/Pagination";
import { useAuth } from "../context/AuthContext";
import { RECONNECTED, useRealtime } from "../context/RealtimeContext";
import { getBranches } from "../services/branches";
import { errorMessage } from "../services/http";
import { getTransfers } from "../services/transfers";
import { formatDateTime, formatSlot } from "../utils/format";
import { STATUSES, STATUS_LABEL, statusClass } from "../utils/transfers";

const SIZE = 20;

// Transferencias que salen de una sucursal o llegan a ella. Se actualiza sola cuando alguna cambia de estado.
function TransfersListPage() {
  const { user } = useAuth();
  const [branches, setBranches] = useState([]);
  const [branchId, setBranchId] = useState(user.branchId ? String(user.branchId) : "");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState({ items: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    getBranches().then(setBranches).catch(() => {});
  }, []);

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await getTransfers({ branchId, status, page, size: SIZE });
      setData({ items: res.items || [], total: res.total });
    } catch (err) {
      setError(errorMessage(err, "No se pudieron cargar las transferencias."));
    } finally {
      setLoading(false);
    }
  }, [branchId, status, page]);

  useEffect(() => {
    setLoading(true);
    load();
  }, [load]);

  useRealtime((event) => {
    if (event.type === RECONNECTED) {
      load();
    } else if (event.type === "TransferenciaActualizada") {
      const { origenId, destinoId } = event.data;
      if (!branchId || String(origenId) === branchId || String(destinoId) === branchId) load();
    }
  });

  function changeFilter(setter) {
    return (e) => {
      setter(e.target.value);
      setPage(1);
    };
  }

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <h1>Transferencias</h1>
        <Link to="/transferencias/nueva" className="btn-primary">
          + Nueva transferencia
        </Link>
      </div>

      <div className="catalog-filters">
        <BranchSelect branches={branches} value={branchId} onChange={changeFilter(setBranchId)} emptyLabel="Todas las sucursales" />
        <select className="category-select" value={status} onChange={changeFilter(setStatus)}>
          <option value="">Todos los estados</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {STATUS_LABEL[s]}
            </option>
          ))}
        </select>
      </div>

      {loading && <p className="state-msg">Cargando…</p>}
      {error && <p className="state-msg error">{error}</p>}
      {!loading && !error && data.items.length === 0 && <p className="state-msg">No hay transferencias.</p>}

      {!loading && !error && data.items.length > 0 && (
        <>
          <table className="admin-table">
            <thead>
              <tr>
                <th>Creada</th>
                <th>Producto</th>
                <th>Cantidad</th>
                <th>Desde</th>
                <th>Hacia</th>
                <th>Estado</th>
                <th>Llega</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((t) => (
                <tr key={t.id}>
                  <td>{formatDateTime(t.createdAt)}</td>
                  <td>{t.productName}</td>
                  <td>{t.quantity}</td>
                  <td>{t.originBranchName}</td>
                  <td>{t.destinationBranchName}</td>
                  <td>
                    <span className={statusClass(t.status)}>{STATUS_LABEL[t.status] || t.status}</span>
                  </td>
                  <td>{formatSlot(t.slotStart, t.slotEnd)}</td>
                  <td className="admin-row-actions">
                    <Link to={`/transferencias/${t.id}`} className="link-action">
                      Ver
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <Pagination page={page} total={data.total} size={SIZE} onChange={setPage} />
        </>
      )}
    </div>
  );
}

export default TransfersListPage;
