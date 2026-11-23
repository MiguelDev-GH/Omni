import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import App from "./App";
import { useAppStore } from "./store/app";
import { json, mockFetch } from "./test/fetch";

const TOKEN = "0123456789abcdef0123456789abcdef";

async function submitToken(token: string) {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Token de acesso"), token);
  await user.click(screen.getByRole("button", { name: "Entrar" }));
}

test("shows token page without token", () => {
  render(<App />);
  expect(screen.getByLabelText("Token de acesso")).toHaveAttribute("type", "password");
  expect(screen.getByRole("switch", { name: "Tema escuro" })).toBeInTheDocument();
});

test("rejects short token locally", async () => {
  const calls = mockFetch(() => json(200, []));
  render(<App />);
  await submitToken("0123456789");
  expect(screen.getByText("O token tem pelo menos 32 caracteres.")).toBeInTheDocument();
  expect(calls).toHaveLength(0);
});

test("logs in with valid token", async () => {
  mockFetch(() => json(200, []));
  render(<App />);
  await submitToken(TOKEN);
  expect(await screen.findByRole("heading", { level: 2, name: "Monitoramento" })).toBeInTheDocument();
  expect(sessionStorage.getItem("omni.token")).toBe(TOKEN);
  expect(localStorage.length).toBe(0);
});

test("shows invalid token", async () => {
  mockFetch(() => json(401, { erro: "não autorizado" }));
  render(<App />);
  await submitToken(TOKEN);
  expect(await screen.findByText("Token inválido.")).toBeInTheDocument();
  expect(screen.getByLabelText("Token de acesso")).toBeInTheDocument();
});

test("shows connection error", async () => {
  mockFetch(() => Promise.reject(new TypeError("rede")));
  render(<App />);
  await submitToken(TOKEN);
  expect(await screen.findByText("Não foi possível falar com o Omni. Ele está rodando?")).toBeInTheDocument();
});

test("works without sessionStorage", async () => {
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("bloqueado");
  });
  mockFetch(() => json(200, []));
  render(<App />);
  await submitToken(TOKEN);
  expect(await screen.findByRole("heading", { level: 2, name: "Monitoramento" })).toBeInTheDocument();
});

test("logout returns to token page", async () => {
  useAppStore.setState({ token: TOKEN });
  mockFetch(() => json(200, []));
  render(<App />);
  await userEvent.setup().click(await screen.findByRole("button", { name: "Sair" }));
  expect(screen.getByLabelText("Token de acesso")).toBeInTheDocument();
  expect(screen.queryByText("Sessão expirada. Informe o token novamente.")).toBeNull();
});

test("shows expired session message", () => {
  useAppStore.setState({ sessionExpired: true });
  render(<App />);
  expect(screen.getByText("Sessão expirada. Informe o token novamente.")).toBeInTheDocument();
});
