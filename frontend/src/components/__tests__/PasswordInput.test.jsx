import { fireEvent, render, screen } from "@testing-library/react";
import PasswordInput from "../PasswordInput";

describe("PasswordInput", () => {
  it("muestra y oculta lo que se escribe", () => {
    render(<PasswordInput placeholder="Contraseña" defaultValue="secreta" />);
    const input = screen.getByPlaceholderText("Contraseña");
    expect(input).toHaveAttribute("type", "password");

    fireEvent.click(screen.getByLabelText("Mostrar la contraseña"));
    expect(input).toHaveAttribute("type", "text");

    fireEvent.click(screen.getByLabelText("Ocultar la contraseña"));
    expect(input).toHaveAttribute("type", "password");
  });
});
