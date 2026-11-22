import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, test } from "vitest";
import type { Scan } from "../api/scans";
import type { Target } from "../api/targets";
import { json, mockFetch } from "../test/fetch";
import ffuf from "./__fixtures__/ffuf.json";
import FfufOptions from "./FfufOptions";
import FfufView, { ffufSummary, seenInTraffic, tree } from "./FfufView";

const scan: Scan = { id: 9, alvo_id: 1, ferramentas: ["ffuf"], status: "concluida", criada_em: "2026-11-21T12:00:00Z", achados: {} };

const routes = [
  { metodo: "GET", rota: "/produtos", requisicoes: 90, erros: 0, p50_ms: 2, p95_ms: 5 },
  { metodo: "GET", rota: "/produtos/:id", requisicoes: 40, erros: 0, p50_ms: 2, p95_ms: 5 },
  { metodo: "GET", rota: "/erro", requisicoes: 5, erros: 5, p50_ms: 1, p95_ms: 1 },
];

const row = (path: string) => screen.getByText(path, { selector: "span.font-mono" }).parentElement!;

test("ffuf groups /admin and /admin/login in the route tree", () => {
  const nodes = tree(ffuf.resultados);
  const admin = nodes.find((n) => n.name === "admin")!;
  expect(admin.result?.status).toBe(301);
  expect(admin.children.map((c) => c.path)).toEqual(["/admin/login"]);
  const git = nodes.find((n) => n.name === ".git")!;
  expect(git.result).toBeUndefined();
  expect(git.children.map((c) => c.name)).toEqual(["config", "HEAD"]);
  expect(ffufSummary(ffuf)).toBe("8 caminhos encontrados");
});

test("ffuf marks only the paths the sniffer never saw", async () => {
  mockFetch((c) => (c.url.startsWith("/api/requisicoes/rotas") ? json(200, routes) : json(500, {})));
  render(<FfufView report={ffuf} findings={[]} scan={scan} />);
  expect(await screen.findByText("5 fora do tráfego real")).toBeInTheDocument();
  expect(row("/.env")).toHaveTextContent("não aparece no tráfego real");
  expect(row("/login")).toHaveTextContent("não aparece no tráfego real");
  expect(row("/produtos")).not.toHaveTextContent("não aparece no tráfego real");
  expect(row("/7")).not.toHaveTextContent("não aparece no tráfego real"); // casa com /produtos/:id
  expect(row("/erro")).toHaveTextContent("500");
  expect(row("/admin")).toHaveTextContent("→ /admin/");
  expect(seenInTraffic("/produtos/7/extra", ["/produtos/:id"])).toBe(false);
});

test("ffuf shows no traffic mark when the routes cannot be loaded", async () => {
  const calls = mockFetch(() => json(500, {}));
  render(<FfufView report={ffuf} findings={[]} scan={scan} />);
  await screen.findByRole("region", { name: "Árvore de rotas" });
  expect(calls).toHaveLength(1);
  expect(screen.queryByText("não aparece no tráfego real")).toBeNull();
});

test("ffuf shows no traffic mark when the sniffer saw no HTTP traffic", async () => {
  mockFetch(() => json(200, []));
  render(<FfufView report={ffuf} findings={[]} scan={scan} />);
  expect(await screen.findByText(/Sem tráfego HTTP registrado/)).toBeInTheDocument();
  expect(screen.queryByText("não aparece no tráfego real")).toBeNull();
});

test("ffuf filters by status class", async () => {
  mockFetch(() => json(200, routes));
  render(<FfufView report={ffuf} findings={[]} scan={scan} />);
  const tree = screen.getByRole("region", { name: "Árvore de rotas" });
  expect(within(tree).getByText("/erro")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("checkbox", { name: /5xx/ }));
  expect(within(tree).queryByText("/erro")).toBeNull();
  await userEvent.click(screen.getByRole("checkbox", { name: /2xx/ }));
  await userEvent.click(screen.getByRole("checkbox", { name: /3xx/ }));
  expect(screen.getByText("Nenhum caminho com estes filtros.")).toBeInTheDocument();
});

test("ffuf options pick the wordlist and one of the target's app ports", async () => {
  const target: Target = { id: 1, nome: "loja", host: "127.0.0.1", portas_app: [3000, 8443], porta_banco: 5433 };
  let last: Record<string, string> = {};
  function Harness() {
    const [v, setV] = useState<Record<string, string>>({});
    return <FfufOptions value={v} onChange={(x) => setV((last = x))} target={target} />;
  }
  render(<Harness />);
  expect(screen.getByRole("combobox", { name: "Porta" })).toHaveTextContent("3000");
  await userEvent.click(screen.getByRole("combobox", { name: "Lista de palavras" }));
  await userEvent.click(screen.getByRole("option", { name: /Média/ }));
  await userEvent.click(screen.getByRole("combobox", { name: "Porta" }));
  expect(screen.queryByRole("option", { name: "5433" })).toBeNull();
  await userEvent.click(screen.getByRole("option", { name: "8443" }));
  expect(last).toEqual({ lista: "media", porta: "8443" });
});
