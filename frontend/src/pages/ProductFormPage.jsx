import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { errorMessage } from "../services/http";
import { createProduct, getProduct, updateProduct } from "../services/products";

// Alta y edición de productos. Solo el director.
function ProductFormPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const editing = Boolean(id);
  const [form, setForm] = useState({ name: "", description: "", supplier: "" });
  const [loading, setLoading] = useState(editing);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!editing) return;
    getProduct(id)
      .then((p) => setForm({ name: p.name, description: p.description || "", supplier: p.supplier || "" }))
      .catch((err) => setError(errorMessage(err, "No se pudo cargar el producto.")))
      .finally(() => setLoading(false));
  }, [id, editing]);

  function handleChange(e) {
    setForm({ ...form, [e.target.name]: e.target.value });
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    const data = { name: form.name.trim(), description: form.description.trim(), supplier: form.supplier.trim() || null };
    try {
      const saved = editing ? await updateProduct(id, data) : await createProduct(data);
      navigate(`/productos/${saved.id || id}`);
    } catch (err) {
      setError(errorMessage(err, "No se pudo guardar el producto."));
      setSaving(false);
    }
  }

  if (loading) return <p className="state-msg">Cargando…</p>;

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <div>
          <Link to="/productos" className="btn-back">
            ← Productos
          </Link>
          <h1>{editing ? "Editar producto" : "Nuevo producto"}</h1>
        </div>
      </div>

      <form className="admin-form" onSubmit={handleSubmit}>
        <label className="form-field">
          <span>Nombre</span>
          <input name="name" value={form.name} onChange={handleChange} required />
        </label>
        <label className="form-field">
          <span>Descripción</span>
          <textarea name="description" value={form.description} onChange={handleChange} rows={3} />
        </label>
        <label className="form-field">
          <span>Proveedor</span>
          <input name="supplier" value={form.supplier} onChange={handleChange} />
        </label>

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

export default ProductFormPage;
