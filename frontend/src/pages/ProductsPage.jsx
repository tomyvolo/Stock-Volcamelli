import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import Pagination from "../components/Pagination";
import { useAuth } from "../context/AuthContext";
import { errorMessage } from "../services/http";
import { getProducts, searchProducts } from "../services/products";
import { isDirector } from "../utils/roles";

const SIZE = 10;

// Catálogo paginado. Con texto, pasa a la búsqueda de Solr, que filtra por proveedor y ordena por relevancia o por nombre.
function ProductsPage() {
  const { user } = useAuth();
  const [form, setForm] = useState({ q: "", supplier: "", sort: "relevancia" });
  const [search, setSearch] = useState(null);
  const [page, setPage] = useState(1);
  const [data, setData] = useState({ items: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError(null);
      try {
        if (search) {
          const res = await searchProducts({ ...search, page, size: SIZE });
          setData({ items: res.results || [], total: res.total });
        } else {
          const res = await getProducts(page, SIZE);
          setData({ items: res.items || [], total: res.total });
        }
      } catch (err) {
        setError(errorMessage(err, "No se pudieron cargar los productos."));
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [search, page]);

  function handleSearch(e) {
    e.preventDefault();
    setPage(1);
    setSearch(form.q.trim() ? { q: form.q.trim(), supplier: form.supplier.trim(), sort: form.sort } : null);
  }

  function handleClear() {
    setForm({ q: "", supplier: "", sort: "relevancia" });
    setSearch(null);
    setPage(1);
  }

  return (
    <div className="admin-shell">
      <div className="admin-header">
        <h1>Productos</h1>
        {isDirector(user) && (
          <Link to="/productos/nuevo" className="btn-primary">
            + Nuevo producto
          </Link>
        )}
      </div>

      <form className="catalog-filters" onSubmit={handleSearch}>
        <input
          className="search-input"
          placeholder="Buscar por nombre o descripción"
          value={form.q}
          onChange={(e) => setForm({ ...form, q: e.target.value })}
        />
        <input
          className="search-input"
          placeholder="Proveedor (opcional)"
          value={form.supplier}
          onChange={(e) => setForm({ ...form, supplier: e.target.value })}
        />
        <select
          className="search-input"
          aria-label="Ordenar por"
          value={form.sort}
          onChange={(e) => setForm({ ...form, sort: e.target.value })}
        >
          <option value="relevancia">Más relevantes</option>
          <option value="nombre">Nombre (A-Z)</option>
        </select>
        <button type="submit" className="btn-primary">
          Buscar
        </button>
        {search && (
          <button type="button" className="btn-secondary" onClick={handleClear}>
            Ver todos
          </button>
        )}
      </form>

      {loading && <p className="state-msg">Cargando…</p>}
      {error && <p className="state-msg error">{error}</p>}
      {!loading && !error && data.items.length === 0 && (
        <p className="state-msg">{search ? "No hay productos que coincidan." : "No hay productos cargados."}</p>
      )}

      {!loading && !error && data.items.length > 0 && (
        <>
          <table className="admin-table">
            <thead>
              <tr>
                <th>Producto</th>
                <th>Proveedor</th>
                {!search && <th>Descripción</th>}
                <th></th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((p) => (
                <tr key={p.id}>
                  <td>{p.name}</td>
                  <td>{p.supplier || "—"}</td>
                  {!search && <td>{p.description || "—"}</td>}
                  <td className="admin-row-actions">
                    <Link to={`/productos/${p.id}`} className="link-action">
                      Stock
                    </Link>
                    {isDirector(user) && (
                      <Link to={`/productos/${p.id}/editar`} className="link-action">
                        Editar
                      </Link>
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

export default ProductsPage;
