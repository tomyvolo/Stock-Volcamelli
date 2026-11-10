import { useState } from "react";

// Campo de contraseña con un botón para ver lo que se está escribiendo. Recibe las mismas props que un input.
function PasswordInput(props) {
  const [visible, setVisible] = useState(false);

  return (
    <div className="password-input">
      <input {...props} type={visible ? "text" : "password"} />
      <button
        type="button"
        className="password-toggle"
        onClick={() => setVisible(!visible)}
        aria-label={visible ? "Ocultar la contraseña" : "Mostrar la contraseña"}
      >
        {visible ? "Ocultar" : "Mostrar"}
      </button>
    </div>
  );
}

export default PasswordInput;
