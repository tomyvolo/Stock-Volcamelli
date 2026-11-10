import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import BranchSelect from "../components/BranchSelect";
import { useAuth } from "../context/AuthContext";
import { RECONNECTED, useRealtime } from "../context/RealtimeContext";
import { getBranches } from "../services/branches";
import { getBranchStock } from "../services/stock";

// Pantalla de inicio: el stock disponible de una sucursal. Arranca en la del usuario y se actualiza sola.
function StockPage() {
  const { user } = useAuth();
  const [branches, setBranches] = useState([]);
  const [branchId, setBranchId] = useState(user.branchId ? String(user.branchId) : "");
  const [lowStock, setLowStock] = useState(false);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [updated, setUpdated] = useState(null);

  useEffect(() => {
    getBranches()
      .then((list) => {
        setBranches(list);
        if (!branchId && list.length > 0) setBranchId(String(list[0].id));
      })
      .catch(() => setError("No se pudieron cargar las sucursales."));
  }, []);

  const load = useCallback(async () => {
    if (!branchId) return;
    setError(null);
    try {
      setItems(await getBranchStock(branchId, lowStock));
    } catch {
      setError("No se pudo cargar el stock. Probá de nuevo en unos minutos.");
    } finally {
      setLoading(false);
    }
  }, [branchId, lowStock]);

  useEffect(() => {
    setLoading(true);
    load();
  }, [load]);

  // Cuando cambia el stock de esta sucursal, recargamos y marcamos la fila que cambió.
  useRealtime((event) => {
    if (event.type === RECONNECTED) {
      load();
    } else if (event.type === "StockActualizado" && String(event.data.sucursalId) === branchId) {
      setUpdated(event.data.productoId);
      load();
    }
  });

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <h1>Stock</h1>
        <div className="catalog-filters">
          <BranchSelect branches={branches} value={branchId} onChange={(e) => setBranchId(e.target.value)} />
          <label className="check-field">
            <input type="checkbox" checked={lowStock} onChange={(e) => setLowStock(e.target.checked)} />
            <span>Solo stock bajo (5 o menos)</span>
          </label>
        </div>
      </div>

      {loading && <p className="state-msg">Cargando…</p>}
      {error && <p className="state-msg error">{error}</p>}
      {!loading && !error && items.length === 0 && (
        <p className="state-msg">{lowStock ? "No hay productos con stock bajo." : "La sucursal no tiene productos."}</p>
      )}

      {!loading && !error && items.length > 0 && (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Producto</th>
              <th>Disponible</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.productId} className={item.productId === updated ? "row-updated" : ""}>
                <td>{item.productName}</td>
                <td>
                  {item.quantity}
                  {item.quantity <= 5 && <span className="tag-cancelled"> · stock bajo</span>}
                </td>
                <td className="admin-row-actions">
                  <Link to={`/sucursales/${branchId}/productos/${item.productId}`} className="link-action">
                    Movimientos
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

export default StockPage;
