import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, test } from "vitest";
import { useAppStore } from "../store/app";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import DiscoveredServices from "./DiscoveredServices";
import TargetForm from "./TargetForm";

const node = { processo: "node", pid: 10, host: "127.0.0.1", tipo: "app", portas: [3000, 9229] };
const db = { host: "127.0.0.1", tipo: "banco", portas: [5432] };
const noop = () => {};

beforeEach(() => {
  useAppStore.setState({ token: "t" });
});

function discovery(body: unknown, status = 200): FetchCall[] {
  return mockFetch((c) => (c.url === "/api/descoberta" ? json(status, body) : json(200, [])));
}

test("lists suggestions", async () => {
  discovery([node, db]);
  render(<DiscoveredServices targets={[]} onPick={noop} />);
  expect(await screen.findByRole("button", { name: /node · 3000, 9229/ })).toBeInTheDocument();
  const unknown = screen.getByRole("button", { name: /Processo desconhecido · 5432/ });
  expect(unknown).toHaveTextContent("banco");
  expect(screen.getByText("Detectado nesta máquina")).toBeInTheDocument();
});

test("hides registered services", async () => {
  discovery([node, db]);
  const registered = [{ id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000, 9229] }];
  render(<DiscoveredServices targets={registered} onPick={noop} />);
  expect(await screen.findByRole("button", { name: /5432/ })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /node/ })).toBeNull();
});

test("shows empty state", async () => {
  discovery([]);
  render(<DiscoveredServices targets={[]} onPick={noop} />);
  expect(await screen.findByText("Nenhum serviço novo encontrado.")).toBeInTheDocument();
});

test("shows error state", async () => {
  discovery({ erro: "não foi possível listar os serviços" }, 500);
  render(<DiscoveredServices targets={[]} onPick={noop} />);
  expect(await screen.findByText("Não foi possível listar os serviços desta máquina.")).toBeInTheDocument();
});

test("fills form from app suggestion", async () => {
  discovery([node, db]);
  render(<TargetForm onDone={noop} onCancel={noop} />);
  await userEvent.setup().click(await screen.findByRole("button", { name: /node · 3000, 9229/ }));
  expect(screen.getByLabelText("Nome")).toHaveValue("node");
  expect(screen.getByLabelText("Host")).toHaveValue("127.0.0.1");
  expect(screen.getByLabelText("Portas da aplicação")).toHaveValue("3000, 9229");
});

test("keeps typed name", async () => {
  discovery([node]);
  render(<TargetForm onDone={noop} onCancel={noop} />);
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Nome"), "minha api");
  await user.click(await screen.findByRole("button", { name: /node/ }));
  expect(screen.getByLabelText("Nome")).toHaveValue("minha api");
});

test("fills database port", async () => {
  discovery([db]);
  render(<TargetForm onDone={noop} onCancel={noop} />);
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Portas da aplicação"), "3000");
  await user.click(await screen.findByRole("button", { name: /5432/ }));
  expect(screen.getByLabelText("Porta do banco (opcional)")).toHaveValue("5432");
  expect(screen.getByLabelText("Portas da aplicação")).toHaveValue("3000");
});

test("edit mode has no suggestions", async () => {
  const calls = discovery([node]);
  render(<TargetForm initial={{ id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000] }} onDone={noop} onCancel={noop} />);
  expect(screen.getByLabelText("Nome")).toHaveValue("api");
  await new Promise((r) => setTimeout(r, 50));
  expect(screen.queryByText("Detectado nesta máquina")).toBeNull();
  expect(calls.some((c) => c.url === "/api/descoberta")).toBe(false);
});
