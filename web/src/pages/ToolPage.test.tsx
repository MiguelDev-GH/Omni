import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";
import type { Finding, Scan, Tool } from "../api/scans";
import gitleaks from "../arsenal/__fixtures__/gitleaks.json";
import nmap from "../arsenal/__fixtures__/nmap.json";
import Layout from "../components/Layout";
import { useAppStore } from "../store/app";
import { FakeWebSocket, installFakeWebSocket } from "../test/fakeSocket";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import ArsenalPage from "./ArsenalPage";
import ToolPage from "./ToolPage";

const target = { id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000] };
const tools: Tool[] = [
  { nome: "nmap", descricao: "Portas abertas e versões dos serviços", imagem: "instrumentisto/nmap:7.95" },
  { nome: "amass", descricao: "Subdomínios", imagem: "x", requer: "dominio" },
  { nome: "gitleaks", descricao: "Segredos vazados no código-fonte", imagem: "zricethezav/gitleaks:v8.30.0", requer: "repositorio" },
];
const done: Scan = { id: 5, alvo_id: 1, ferramentas: ["nmap"], status: "concluida", criada_em: "2026-11-21T12:00:00Z", achados: { info: 1 } };
const finding: Finding = { ferramenta: "nmap", titulo: "Porta 3000/tcp aberta: http", evidencia: "", severidade: "info", porta: 3000, assinatura: "a" };
const noChanges = { novos: [], resolvidos: [], anteriores: {} };

function renderAt(path: string, handler: (c: FetchCall) => Response | undefined = () => undefined, scans: Scan[] = [done]) {
  const calls = mockFetch((c) => {
    const custom = handler(c);
    if (custom) return custom;
    if (c.url === "/api/ws/ticket") return json(200, { ticket: "t" });
    if (c.url === "/api/alvos") return json(200, [target]);
    if (c.url === "/api/ferramentas") return json(200, tools);
    if (c.url.startsWith("/api/varreduras?")) return json(200, scans);
    if (c.url === "/api/varreduras/5/achados") return json(200, [finding]);
    if (c.url === "/api/varreduras/5/comparacao") return json(200, noChanges);
    return json(500, { erro: "inesperado" });
  });
  installFakeWebSocket();
  useAppStore.setState({ token: "t" });
  const router = createMemoryRouter(
    [
      {
        element: <Layout />,
        children: [
          { path: "/arsenal", element: <ArsenalPage /> },
          { path: "/arsenal/:ferramenta", element: <ToolPage /> },
        ],
      },
    ],
    { initialEntries: [path] },
  );
  render(<RouterProvider router={router} />);
  return calls;
}

test("/arsenal/nmap shows the Nmap view from the report", async () => {
  renderAt("/arsenal/nmap", (c) => (c.url === "/api/varreduras/5/relatorio/nmap" ? json(200, nmap) : undefined));
  expect(await screen.findByRole("heading", { level: 1, name: "Nmap" })).toBeInTheDocument();
  const table = await screen.findByRole("table", { name: "Mapa de portas" });
  expect(within(table).getByText("39999/tcp")).toBeInTheDocument();
  expect(screen.getByText("Saída da ferramenta").closest("details")).not.toHaveAttribute("open");
});

test("falls back to findings without a report", async () => {
  renderAt("/arsenal/nmap", (c) => (c.url === "/api/varreduras/5/relatorio/nmap" ? json(404, { erro: "relatório indisponível" }) : undefined));
  expect(await screen.findByText("Relatório indisponível para esta varredura.")).toBeInTheDocument();
  const table = screen.getByRole("table", { name: "Achados" });
  expect(within(table).getByText("Porta 3000/tcp aberta: http")).toBeInTheDocument();
});

test("starts a scan of the tool and opens the live output while it runs", async () => {
  const scans: Scan[] = [];
  const calls = renderAt(
    "/arsenal/nmap",
    (c) => {
      if (c.method === "POST" && c.url === "/api/varreduras") {
        scans.unshift({ ...done, status: "rodando" });
        return json(202, { id: 5 });
      }
      return undefined;
    },
    scans,
  );
  expect(await screen.findByText("Nenhuma varredura do Nmap para este alvo.")).toBeInTheDocument();
  await waitFor(() => expect(document.getElementById("alvo-nmap")).toHaveTextContent("api"));
  await userEvent.click(screen.getByRole("button", { name: "Iniciar varredura" }));
  const post = calls.find((c) => c.method === "POST" && c.url === "/api/varreduras");
  expect(JSON.parse(post!.body!)).toEqual({ alvo_id: 1, ferramentas: ["nmap"] });
  expect(await screen.findByText(/A varredura ainda está rodando/)).toBeInTheDocument();
  expect(screen.getByText("Saída da ferramenta").closest("details")).toHaveAttribute("open");

  const ws = FakeWebSocket.instances.at(-1)!;
  act(() => ws.open());
  act(() => ws.message({ tipo: "SCAN_LOG", dados: { scan_id: 5, ferramenta: "nmap", linhas: ["Starting Nmap"] } }));
  expect(screen.getByRole("log")).toHaveTextContent("Starting Nmap");
});

test("unknown tool shows the not found page", async () => {
  renderAt("/arsenal/bash");
  expect(await screen.findByText(/não existe|não encontrada/i)).toBeInTheDocument();
});

test("overview disables a tool that needs the target domain", async () => {
  renderAt("/arsenal", (c) => (c.url === "/api/varreduras/5/relatorio/nmap" ? json(200, nmap) : undefined));
  const amass = await screen.findByRole("checkbox", { name: /amass/ });
  expect(amass).toBeDisabled();
  expect(screen.getByText("Precisa do domínio do alvo")).toBeInTheDocument();
  expect(await screen.findByText("2 portas abertas de 3")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Abrir página do Nmap" })).toHaveAttribute("href", "/arsenal/nmap");
});

test("/arsenal/vazamentos is the Gitleaks page", async () => {
  const leaks: Scan = { ...done, id: 6, ferramentas: ["gitleaks"] };
  renderAt(
    "/arsenal/vazamentos",
    (c) => {
      if (c.url === "/api/varreduras/6/relatorio/gitleaks") return json(200, gitleaks);
      if (c.url === "/api/varreduras/6/achados") return json(200, []);
      if (c.url === "/api/varreduras/6/comparacao") return json(200, noChanges);
      return undefined;
    },
    [leaks],
  );
  expect(await screen.findByRole("heading", { level: 1, name: "Vazamentos" })).toBeInTheDocument();
  expect(await screen.findByText(/feito com o Gitleaks/)).toBeInTheDocument();
  expect(await screen.findByRole("listitem", { name: "config/aws.env" })).toBeInTheDocument();
});

test("/arsenal/gitleaks shows the not found page", async () => {
  renderAt("/arsenal/gitleaks");
  expect(await screen.findByText(/não encontrada|não existe/i)).toBeInTheDocument();
});
