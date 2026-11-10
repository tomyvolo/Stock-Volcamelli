import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { RECONNECTED, useRealtime } from "../context/RealtimeContext";
import { errorMessage } from "../services/http";
import { cancelTransfer, getTransfer } from "../services/transfers";
import { formatDateTime, formatSlot } from "../utils/format";
import { STATUS_LABEL, canCancel, statusClass } from "../utils/transfers";

// Detalle de una transferencia: estado, franja de Rumbo, historial y cancelación. Se actualiza sola.
function TransferDetailPage() {
  const { id } = useParams();
  const [transfer, setTransfer] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [reason, setReason] = useState("");
  const [cancelling, setCancelling] = useState(false);
  const [msg, setMsg] = useState(null);

  const load = useCallback(async () => {
    try {
      setTransfer(await getTransfer(id));
      setError(null);
    } catch (err) {
      setError(errorMessage(err, "No se pudo cargar la transferencia."));
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  // El aviso no trae todos los datos: cuando cambia esta transferencia, la volvemos a pedir (ADR-019).
  useRealtime((event) => {
    if (event.type === RECONNECTED || (event.type === "TransferenciaActualizada" && event.data.transferenciaId === id)) {
      load();
    }
  });

  async function handleCancel(e) {
    e.preventDefault();
    if (!confirm("¿Cancelar la transferencia?")) return;
    setCancelling(true);
    setMsg(null);
    try {
      setTransfer(await cancelTransfer(id, reason.trim()));
      setReason("");
    } catch (err) {
      setMsg(errorMessage(err, "No se pudo cancelar la transferencia."));
    } finally {
      setCancelling(false);
    }
  }

  if (loading) return <p className="state-msg">Cargando…</p>;
  if (error) return <p className="state-msg error">{error}</p>;

  const t = transfer;
  return (
    <div className="admin-shell">
      <div className="admin-header">
        <div>
          <Link to="/transferencias" className="btn-back">
            ← Transferencias
          </Link>
          <h1>
            {t.quantity} × {t.productName}
          </h1>
          <p className="report-subtitle">
            {t.originBranchName} → {t.destinationBranchName}
          </p>
        </div>
        <span className={statusClass(t.status)}>{STATUS_LABEL[t.status] || t.status}</span>
      </div>

      <div className="detail-card">
        <dl>
          <div className="detail-row">
            <dt>Llega</dt>
            <dd>{t.slotStart ? formatSlot(t.slotStart, t.slotEnd) : "Todavía no está reservada en Rumbo"}</dd>
          </div>
          {t.delayMinutes > 0 && (
            <div className="detail-row">
              <dt>Demora</dt>
              <dd>{t.delayMinutes} minutos</dd>
            </div>
          )}
          <div className="detail-row">
            <dt>Hora pedida</dt>
            <dd>{t.desiredArrival ? formatDateTime(t.desiredArrival) : "Sin hora pedida"}</dd>
          </div>
          {t.notes && (
            <div className="detail-row">
              <dt>Observaciones</dt>
              <dd>{t.notes}</dd>
            </div>
          )}
          {t.reason && (
            <div className="detail-row">
              <dt>Motivo</dt>
              <dd>{t.reason}</dd>
            </div>
          )}
          <div className="detail-row">
            <dt>Creada</dt>
            <dd>{formatDateTime(t.createdAt)}</dd>
          </div>
        </dl>
      </div>

      {t.cancellationRequested && (
        <p className="state-msg">La cancelación está pedida y se le está mandando a Rumbo.</p>
      )}

      {canCancel(t) && (
        <form className="admin-form" onSubmit={handleCancel}>
          <h2>Cancelar</h2>
          <label className="form-field">
            <span>Motivo</span>
            <input value={reason} onChange={(e) => setReason(e.target.value)} required maxLength={500} />
          </label>
          {msg && <p className="state-msg error">{msg}</p>}
          <div className="form-actions">
            <button type="submit" className="btn-danger" disabled={cancelling}>
              {cancelling ? "Cancelando…" : "Cancelar transferencia"}
            </button>
          </div>
        </form>
      )}

      <h2>Historial</h2>
      <table className="admin-table">
        <thead>
          <tr>
            <th>Fecha</th>
            <th>Estado</th>
            <th>Detalle</th>
          </tr>
        </thead>
        <tbody>
          {(t.history || []).map((h, i) => (
            <tr key={i}>
              <td>{formatDateTime(h.date)}</td>
              <td>
                <span className={statusClass(h.status)}>{STATUS_LABEL[h.status] || h.status}</span>
              </td>
              <td>{h.detail || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default TransferDetailPage;
