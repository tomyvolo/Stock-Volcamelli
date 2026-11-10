import { parseEvents } from "../events";

describe("parseEvents", () => {
  it("separa los mensajes de Server-Sent Events", () => {
    const text =
      'event: StockActualizado\ndata: {"sucursalId":1,"productoId":4,"disponible":6}\n\n' +
      'event: TransferenciaActualizada\ndata: {"transferenciaId":"abc","estado":"PROGRAMADA"}\n\n';

    const { events, rest } = parseEvents(text);

    expect(events).toEqual([
      { type: "StockActualizado", data: { sucursalId: 1, productoId: 4, disponible: 6 } },
      { type: "TransferenciaActualizada", data: { transferenciaId: "abc", estado: "PROGRAMADA" } },
    ]);
    expect(rest).toBe("");
  });

  // Lo que llega cortado por la red se completa con lo próximo que llegue.
  it("guarda el mensaje a medias para la próxima lectura", () => {
    const first = parseEvents('event: StockActualizado\ndata: {"sucursalId":1');
    expect(first.events).toEqual([]);

    const second = parseEvents(first.rest + ',"productoId":4}\n\n');
    expect(second.events).toEqual([{ type: "StockActualizado", data: { sucursalId: 1, productoId: 4 } }]);
  });

  // Tiempo Real manda ":" cada 25 segundos para que la conexión no se corte.
  it("ignora los avisos de conexión viva", () => {
    const { events } = parseEvents(":\n\n:\n\n");

    expect(events).toEqual([]);
  });
});
