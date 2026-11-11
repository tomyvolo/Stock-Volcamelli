import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import ChatBubble from "../ChatBubble";
import { confirmTransfer, sendMessage } from "../../services/assistant";

vi.mock("../../services/assistant", () => ({
  sendMessage: vi.fn(),
  confirmTransfer: vi.fn(),
}));

const CARD = {
  productId: 7,
  productName: "Harina 000",
  quantity: 4,
  originBranchId: 1,
  originBranchName: "Centro",
  destinationBranchId: 2,
  destinationBranchName: "Nueva Córdoba",
};

function ask(text) {
  fireEvent.change(screen.getByPlaceholderText("Escribí tu pregunta"), { target: { value: text } });
  fireEvent.click(screen.getByText("Enviar"));
}

describe("ChatBubble", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    render(<ChatBubble />);
    fireEvent.click(screen.getByLabelText("Abrir el asistente"));
  });

  it("muestra la respuesta del asistente", async () => {
    sendMessage.mockResolvedValue({ message: "En Centro quedan 12 unidades." });

    ask("¿Cuánta harina queda en Centro?");

    expect(await screen.findByText("En Centro quedan 12 unidades.")).toBeInTheDocument();
    expect(sendMessage).toHaveBeenCalledWith([{ role: "user", content: "¿Cuánta harina queda en Centro?" }]);
  });

  // La transferencia se crea recién cuando el usuario toca Confirmar.
  it("crea la transferencia solo al confirmar la tarjeta", async () => {
    sendMessage.mockResolvedValue({ message: "¿Confirmás esta transferencia?", confirmation: CARD });
    confirmTransfer.mockResolvedValue({ message: "Listo, creé la transferencia abc." });

    ask("Pasá 4 harinas de Centro a Nueva Córdoba");
    expect(await screen.findByText("Nueva Córdoba")).toBeInTheDocument();
    expect(confirmTransfer).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText("Confirmar"));

    await waitFor(() => expect(confirmTransfer).toHaveBeenCalledWith(CARD));
    expect(await screen.findByText("Listo, creé la transferencia abc.")).toBeInTheDocument();
  });

  it("no crea nada si el usuario cancela la tarjeta", async () => {
    sendMessage.mockResolvedValue({ message: "¿Confirmás esta transferencia?", confirmation: CARD });

    ask("Pasá 4 harinas de Centro a Nueva Córdoba");
    fireEvent.click(await screen.findByText("Cancelar"));

    expect(await screen.findByText("Listo, no creé la transferencia.")).toBeInTheDocument();
    expect(confirmTransfer).not.toHaveBeenCalled();
  });

  it("avisa si el asistente no está disponible", async () => {
    sendMessage.mockRejectedValue({ response: { data: { message: "El asistente no está disponible en este momento." } } });

    ask("Hola");

    expect(await screen.findByText("El asistente no está disponible en este momento.")).toBeInTheDocument();
  });
});
