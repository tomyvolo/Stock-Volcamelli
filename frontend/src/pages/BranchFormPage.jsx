import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { createBranch, getBranch, updateBranch } from "../services/branches";
import { errorMessage } from "../services/http";

// Alta y edición de sucursales. Solo el director.
function BranchFormPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const editing = Boolean(id);
  const [form, setForm] = useState({ name: "", address: "" });
  const [loading, setLoading] = useState(editing);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!editing) return;
    getBranch(id)
      .then((b) => setForm({ name: b.name, address: b.address }))
      .catch((err) => setError(errorMessage(err, "No se pudo cargar la sucursal.")))
      .finally(() => setLoading(false));
  }, [id, editing]);

  function handleChange(e) {
    setForm({ ...form, [e.target.name]: e.target.value });
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    const data = { name: form.name.trim(), address: form.address.trim() };
    try {
      if (editing) await updateBranch(id, data);
      else await createBranch(data);
      navigate("/sucursales");
    } catch (err) {
      setError(errorMessage(err, "No se pudo guardar la sucursal."));
      setSaving(false);
    }
  }

  if (loading) return <p className="state-msg">Cargando…</p>;

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <div>
          <Link to="/sucursales" className="btn-back">
            ← Sucursales
          </Link>
          <h1>{editing ? "Editar sucursal" : "Nueva sucursal"}</h1>
        </div>
      </div>

      <form className="admin-form" onSubmit={handleSubmit}>
        <label className="form-field">
          <span>Nombre</span>
          <input name="name" value={form.name} onChange={handleChange} required />
        </label>
        <label className="form-field">
          <span>Dirección</span>
          <input name="address" value={form.address} onChange={handleChange} required />
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

export default BranchFormPage;
