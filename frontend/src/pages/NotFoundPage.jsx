import { Link } from "react-router-dom";

function NotFoundPage() {
  return (
    <div className="notfound-shell">
      <h1>404</h1>
      <p>No encontramos esa página.</p>
      <Link to="/" className="btn-primary">
        Volver al stock
      </Link>
    </div>
  );
}

export default NotFoundPage;
