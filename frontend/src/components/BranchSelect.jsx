// BranchSelect es el selector de sucursal que usan varias pantallas. emptyLabel agrega una opción vacía.
function BranchSelect({ branches, value, onChange, emptyLabel, name, required }) {
  return (
    <select className="category-select" name={name} value={value} onChange={onChange} required={required}>
      {emptyLabel !== undefined && <option value="">{emptyLabel}</option>}
      {branches.map((b) => (
        <option key={b.id} value={b.id}>
          {b.name}
        </option>
      ))}
    </select>
  );
}

export default BranchSelect;
