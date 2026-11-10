function Pagination({ page, total, size, onChange }) {
  const pages = Math.max(1, Math.ceil(total / size));
  return (
    <div className="pagination">
      <button type="button" className="btn-secondary" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        ← Anterior
      </button>
      <span>
        Página {page} de {pages}
      </span>
      <button type="button" className="btn-secondary" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        Siguiente →
      </button>
    </div>
  );
}

export default Pagination;
