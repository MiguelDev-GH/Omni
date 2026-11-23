import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it } from "vitest";
import { useAppStore } from "../store/app";
import Topbar from "./Topbar";

describe("Topbar: tema", () => {
  beforeEach(() => {
    localStorage.clear();
    useAppStore.setState({ tema: "claro" });
    delete document.documentElement.dataset.tema;
  });

  it("alterna entre claro e escuro, aplica no html e guarda a escolha", async () => {
    render(
      <MemoryRouter>
        <Topbar menuOpen={false} onToggleMenu={() => {}} />
      </MemoryRouter>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Usar tema escuro" }));
    expect(document.documentElement.dataset.tema).toBe("escuro");
    expect(localStorage.getItem("omni.tema")).toBe("escuro");

    await userEvent.click(screen.getByRole("button", { name: "Usar tema claro" }));
    expect(document.documentElement.dataset.tema).toBe("claro");
    expect(localStorage.getItem("omni.tema")).toBe("claro");
  });
});
