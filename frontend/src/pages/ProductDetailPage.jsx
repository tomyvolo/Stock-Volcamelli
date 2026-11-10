import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { RECONNECTED, useRealtime } from "../context/RealtimeContext";
import { getBranches } from "../services/branches";
import { errorMessage } from "../services/http";
import { getProduct } from "../services/products";
import { getProductStock } from "../services/stock";

// Un producto y su stock disponible en cada sucursal.
function ProductDetailPage() {
  const { id } = useParams();
  const [product, setProduct] = useState(null);
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [p, branches] = await Promise.all([getProduct(id), getBranches()]);
      setProduct(p);
      // Una sucursal sin ese producto responde 404: la mostramos con 0.
      const stock = await Promise.all(
        branches.map((b) =>
          getProductStock(b.id, id)
            .then((s) => s.quantity)
            .catch((err) => (err.response?.status === 404 ? 0 : null))
        )
      );
      setRows(branches.map((b, i) => ({ branch: b, quantity: stock[i] })));
    } catch (err) {
      setError(errorMessage(err, "No se pudo cargar el producto."));
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  useRealtime((event) => {
    if (event.type === RECONNECTED || (event.type === "StockActualizado" && String(event.data.productoId) === id)) {
      load();
    }
  });

  if (loading) return <p className="state-msg">Cargando…</p>;
  if (error) return <p className="state-msg error">{error}</p>;

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <div>
          <Link to="/productos" className="btn-back">
            ← Productos
          </Link>
          <h1>{product.name}</h1>
          <p className="report-subtitle">
            {product.supplier || "Sin proveedor"} · {product.description || "Sin descripción"}
          </p>
        </div>
      </div>

      <table className="admin-table">
        <thead>
          <tr>
            <th>Sucursal</th>
            <th>Disponible</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ branch, quantity }) => (
            <tr key={branch.id}>
              <td>{branch.name}</td>
              <td>{quantity === null ? "No disponible" : quantity}</td>
              <td className="admin-row-actions">
                <Link to={`/sucursales/${branch.id}/productos/${id}`} className="link-action">
                  Movimientos
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default ProductDetailPage;
