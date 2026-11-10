import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { openEvents } from "../services/events";
import { useAuth } from "./AuthContext";

// Una sola conexión a /api/events para toda la app. Cada pantalla se suscribe a los avisos que le interesan.
export const RealtimeContext = createContext(null);

// Aviso que mandamos a las pantallas cuando la conexión vuelve después de cortarse: los eventos de ese rato
// no se recuperan, así que recargan sus datos (ADR-019).
export const RECONNECTED = "Reconectado";

export function RealtimeProvider({ children }) {
  const { isAuthenticated } = useAuth();
  const [connected, setConnected] = useState(false);
  const listeners = useRef(new Set());

  useEffect(() => {
    if (!isAuthenticated) return undefined;
    let wasConnected = false;
    let lost = false;
    const notify = (event) => listeners.current.forEach((listener) => listener(event));

    const close = openEvents(notify, (ok) => {
      setConnected(ok);
      if (ok && wasConnected && lost) notify({ type: RECONNECTED, data: {} });
      if (ok) wasConnected = true;
      lost = !ok;
    });
    return () => {
      close();
      setConnected(false);
    };
  }, [isAuthenticated]);

  const subscribe = useCallback((listener) => {
    listeners.current.add(listener);
    return () => listeners.current.delete(listener);
  }, []);
  const value = useMemo(() => ({ connected, subscribe }), [connected, subscribe]);

  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>;
}

// useRealtime llama a handler con cada aviso mientras la pantalla está abierta, y devuelve si hay conexión.
export function useRealtime(handler) {
  const ctx = useContext(RealtimeContext);
  if (!ctx) {
    throw new Error("useRealtime debe usarse dentro de un RealtimeProvider");
  }
  const handlerRef = useRef(handler);
  handlerRef.current = handler;

  const { subscribe } = ctx;
  useEffect(() => subscribe((event) => handlerRef.current(event)), [subscribe]);
  return ctx.connected;
}
