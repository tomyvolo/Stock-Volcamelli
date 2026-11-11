import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import BranchSelect from "../components/BranchSelect";
import { useAuth } from "../context/AuthContext";
import { getBranches } from "../services/branches";
import { errorMessage } from "../services/http";
import { searchProducts } from "../services/products";
import { createTransfer } from "../services/transfers";

// Nueva transferencia. El producto se elige buscándolo. La hora de llegada es opcional: Transferencias reserva
// en Rumbo la franja que la contiene o, si no tiene lugar o no se eligió hora, la primera con lugar (ADR-009).
function TransferFormPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [branches, setBranches] = useState([]);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState([]);
  const [product, setProduct] = useState(null);
  const [form, setForm] = useState({
    quantity: "",
    originBranchId: user.branchId ? String(user.branchId) : "",
    destinationBranchId: "",
    desiredArrival: "",
    notes: "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    getBranches().then(setBranches).catch(() => setError("No se pudieron cargar las sucursales."));
  }, []);

  async function handleSearch(e) {
    e.preventDefault();
    if (!query.trim()) return;
    try {
      const res = await searchProducts({ q: query.trim(), size: 8 });
      setResults(res.results || []);
    } catch (err) {
      setError(errorMessage(err, "No se pudo buscar el producto."));
    }
  }

  function handleChange(e) {
    setForm({ ...form, [e.target.name]: e.target.value });
  }

  async function handleSubmit(e) {
    e.preventDefault();
    if (!product) {
      setError("Elegí el producto.");
      return;
    }
    setSaving(true);
    setError(null);
    const data = {
      productId: product.id,
      quantity: Number(form.quantity),
      originBranchId: Number(form.originBranchId),
      destinationBranchId: Number(form.destinationBranchId),
      notes: form.notes.trim(),
    };
    // datetime-local no trae zona horaria: la convertimos a ISO 8601 con la hora del navegador.
    if (form.desiredArrival) data.desiredArrival = new Date(form.desiredArrival).toISOString();
    try {
      const created = await createTransfer(data);
      navigate(`/transferencias/${created.id}`);
    } catch (err) {
      setError(errorMessage(err, "No se pudo crear la transferencia."));
      setSaving(false);
    }
  }

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <div>
          <Link to="/transferencias" className="btn-back">
            ← Transferencias
          </Link>
          <h1>Nueva transferencia</h1>
        </div>
      </div>

      <form className="catalog-filters" onSubmit={handleSearch}>
        <input
          className="search-input"
          placeholder="Buscar el producto"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <button type="submit" className="btn-secondary">
          Buscar
        </button>
      </form>
      {results.length > 0 && (
        <table className="admin-table">
          <tbody>
            {results.map((p) => (
              <tr key={p.id} className={product?.id === p.id ? "row-updated" : ""}>
                <td>{p.name}</td>
                <td>{p.supplier || "—"}</td>
                <td className="admin-row-actions">
                  <button type="button" className="link-action" onClick={() => setProduct(p)}>
                    Elegir
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <form className="admin-form" onSubmit={handleSubmit}>
        <p className="report-subtitle">Producto: {product ? <strong>{product.name}</strong> : "buscalo y elegilo arriba"}</p>
        <div className="form-row">
          <label className="form-field">
            <span>Cantidad</span>
            <input type="number" min="1" name="quantity" value={form.quantity} onChange={handleChange} required />
          </label>
          <label className="form-field">
            <span>Desde</span>
            <BranchSelect
              branches={branches}
              name="originBranchId"
              value={form.originBranchId}
              onChange={handleChange}
              emptyLabel="Elegí el origen"
              required
            />
          </label>
          <label className="form-field">
            <span>Hacia</span>
            <BranchSelect
              branches={branches}
              name="destinationBranchId"
              value={form.destinationBranchId}
              onChange={handleChange}
              emptyLabel="Elegí el destino"
              required
            />
          </label>
        </div>
        <label className="form-field">
          <span>Hora de llegada (opcional)</span>
          <input type="datetime-local" name="desiredArrival" value={form.desiredArrival} onChange={handleChange} />
        </label>
        <label className="form-field">
          <span>Observaciones para el retiro o la entrega (opcional)</span>
          <input name="notes" value={form.notes} onChange={handleChange} maxLength={500} />
        </label>

        {error && <p className="state-msg error">{error}</p>}

        <div className="form-actions">
          <button type="submit" className="btn-primary" disabled={saving}>
            {saving ? "Creando…" : "Crear transferencia"}
          </button>
        </div>
      </form>
    </div>
  );
}

export default TransferFormPage;
