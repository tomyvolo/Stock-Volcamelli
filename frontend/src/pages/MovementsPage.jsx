import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import Pagination from "../components/Pagination";
import { useAuth } from "../context/AuthContext";
import { RECONNECTED, useRealtime } from "../context/RealtimeContext";
import { errorMessage } from "../services/http";
import { adjustStock, getMovements, getProductStock } from "../services/stock";
import { formatDateTime } from "../utils/format";
import { canAdjust } from "../utils/roles";

const SIZE = 20;
const EMPTY_ADJUSTMENT = { type: "ENTRADA", quantity: "", reason: "" };

// Historial de movimientos de un producto en una sucursal. El director, o el admin de esa sucursal, hace ajustes.
function MovementsPage() {
  const { branchId, productId } = useParams();
  const { user } = useAuth();
  const [stock, setStock] = useState(null);
  const [data, setData] = useState({ items: [], total: 0 });
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [form, setForm] = useState(EMPTY_ADJUSTMENT);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [s, m] = await Promise.all([
        getProductStock(branchId, productId).catch((err) => (err.response?.status === 404 ? null : Promise.reject(err))),
        getMovements(branchId, productId, page, SIZE),
      ]);
      setStock(s);
      setData({ items: m.items || [], total: m.total });
    } catch (err) {
      setError(errorMessage(err, "No se pudieron cargar los movimientos."));
    } finally {
      setLoading(false);
    }
  }, [branchId, productId, page]);

  useEffect(() => {
    load();
  }, [load]);

  useRealtime((event) => {
    const mine =
      event.type === "StockActualizado" &&
      String(event.data.sucursalId) === branchId &&
      String(event.data.productoId) === productId;
    if (mine || event.type === RECONNECTED) load();
  });

  async function handleAdjust(e) {
    e.preventDefault();
    setSaving(true);
    setMsg(null);
    try {
      await adjustStock(branchId, productId, { ...form, quantity: Number(form.quantity) });
      setForm(EMPTY_ADJUSTMENT);
      setMsg({ type: "success", text: "Ajuste registrado." });
      setPage(1);
      await load();
    } catch (err) {
      setMsg({ type: "error", text: errorMessage(err, "No se pudo registrar el ajuste.") });
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <p className="state-msg">Cargando…</p>;
  if (error) return <p className="state-msg error">{error}</p>;

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <div>
          <Link to="/" className="btn-back">
            ← Stock
          </Link>
          <h1>{stock?.productName || `Producto ${productId}`}</h1>
          <p className="report-subtitle">
            Sucursal {branchId} · {stock ? `${stock.quantity} disponibles` : "sin stock en esta sucursal"}
          </p>
        </div>
      </div>

      {canAdjust(user, branchId) && (
        <form className="admin-form" onSubmit={handleAdjust}>
          <h2>Ajuste manual</h2>
          <div className="form-row">
            <label className="form-field">
              <span>Tipo</span>
              <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                <option value="ENTRADA">Entrada</option>
                <option value="SALIDA">Salida</option>
              </select>
            </label>
            <label className="form-field">
              <span>Cantidad</span>
              <input
                type="number"
                min="1"
                value={form.quantity}
                onChange={(e) => setForm({ ...form, quantity: e.target.value })}
                required
              />
            </label>
          </div>
          <label className="form-field">
            <span>Motivo</span>
            <input
              value={form.reason}
              onChange={(e) => setForm({ ...form, reason: e.target.value })}
              placeholder="Por ejemplo: llegó mercadería del proveedor"
              required
            />
          </label>
          {msg && <p className={`state-msg ${msg.type}`}>{msg.text}</p>}
          <div className="form-actions">
            <button type="submit" className="btn-primary" disabled={saving}>
              {saving ? "Guardando…" : "Registrar ajuste"}
            </button>
          </div>
        </form>
      )}

      <h2>Movimientos</h2>
      {data.items.length === 0 ? (
        <p className="state-msg">Todavía no hay movimientos.</p>
      ) : (
        <>
          <table className="admin-table">
            <thead>
              <tr>
                <th>Fecha</th>
                <th>Tipo</th>
                <th>Cantidad</th>
                <th>Total después</th>
                <th>Motivo</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((m) => (
                <tr key={m.id}>
                  <td>{formatDateTime(m.createdAt)}</td>
                  <td>
                    <span className={m.type === "ENTRADA" ? "tag-active" : "tag-cancelled"}>
                      {m.type === "ENTRADA" ? "Entrada" : "Salida"}
                    </span>
                  </td>
                  <td>{m.quantity}</td>
                  <td>{m.resultingTotal}</td>
                  <td>
                    {m.reason === "TRANSFERENCIA" && m.transferId ? (
                      <Link to={`/transferencias/${m.transferId}`} className="link-action">
                        {m.description || "Transferencia"}
                      </Link>
                    ) : (
                      m.description || "Ajuste"
                    )}
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

export default MovementsPage;
