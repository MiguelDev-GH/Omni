import { afterEach, describe, expect, it, vi } from "vitest";
import { initialTema } from "./tema";

describe("initialTema", () => {
  afterEach(() => localStorage.clear());

  it("usa a escolha guardada", () => {
    localStorage.setItem("omni.tema", "escuro");
    expect(initialTema()).toBe("escuro");
  });

  it("sem escolha, segue a preferência do sistema", () => {
    vi.stubGlobal("matchMedia", () => ({ matches: true }));
    expect(initialTema()).toBe("escuro");
    vi.stubGlobal("matchMedia", () => ({ matches: false }));
    expect(initialTema()).toBe("claro");
  });

  it("ignora valor guardado inválido", () => {
    localStorage.setItem("omni.tema", "roxo");
    vi.stubGlobal("matchMedia", () => ({ matches: false }));
    expect(initialTema()).toBe("claro");
  });
});
