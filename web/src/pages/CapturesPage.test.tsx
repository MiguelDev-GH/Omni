import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test, vi } from "vitest";
import type { Capture, CapturePage } from "../api/captures";
import { useAppStore } from "../store/app";
import { json, mockFetch, type FetchCall } from "../test/fetch";
import CaptureInspectorPage from "./CaptureInspectorPage";
import CapturesPage, { formatBytes } from "./CapturesPage";

const done: Capture = {
  id: 3,
  inicio: "2026-11-26T12:00:00Z",
  fim: "2026-11-26T12:01:00Z",
  duracao_s: 60,
  porta: 0,
  pacotes: 1200,
  bytes: 2 * 1024 * 1024,
  descartados: 0,
  status: "concluida",
};

function renderAt(path: string, handler: (c: FetchCall) => Response | undefined = () => undefined) {
  const calls = mockFetch((c) => handler(c) ?? (c.url === "/api/capturas" ? json(200, [done]) : json(500, { erro: "inesperado" })));
  useAppStore.setState({ token: "t" });
  useAppStore.getState().setEngine("sniffer", "ativo");
  useAppStore.getState().setTargets([{ id: 1, nome: "api", host: "127.0.0.1", portas_app: [3000], porta_banco: 5432 }]);
  const router = createMemoryRouter(
    [
      { path: "/capturas", element: <CapturesPage /> },
      { path: "/inspetor/captura/:id", element: <CaptureInspectorPage /> },
    ],
    { initialEntries: [path] },
  );
  render(<RouterProvider router={router} />);
  return { calls, router };
}

test("lists captures with the privacy notice and actions", async () => {
  renderAt("/capturas");
  const row = (await screen.findByText("1.200")).closest("tr")!;
  expect(within(row).getByText("2 MiB")).toBeInTheDocument();
  expect(within(row).getByText("Concluída")).toBeInTheDocument();
  expect(within(row).getByText("todas")).toBeInTheDocument();
  expect(within(row).getByRole("link", { name: "Abrir a captura 3 no Inspetor" })).toHaveAttribute(
    "href",
    "/inspetor/captura/3",
  );
  expect(screen.getByText(/dados\s+pessoais/)).toBeInTheDocument();
});

test("records with the chosen duration and port", async () => {
  const user = userEvent.setup();
  const { calls } = renderAt("/capturas", (c) => (c.method === "POST" ? json(202, { id: 4 }) : undefined));
  await screen.findByText("1.200");

  await user.clear(screen.getByLabelText("Duração (segundos)"));
  await user.type(screen.getByLabelText("Duração (segundos)"), "5");
  await user.click(screen.getByRole("button", { name: "Gravar captura" }));
  expect(screen.getByRole("alert")).toHaveTextContent("10 a 300");
  expect(calls.some((c) => c.method === "POST")).toBe(false);

  await user.clear(screen.getByLabelText("Duração (segundos)"));
  await user.type(screen.getByLabelText("Duração (segundos)"), "30");
  await user.click(screen.getByRole("combobox", { name: "Porta" }));
  await user.click(screen.getByRole("option", { name: "Porta 5432" }));
  await user.click(screen.getByRole("button", { name: "Gravar captura" }));
  await waitFor(() => expect(calls.find((c) => c.method === "POST")?.body).toBe('{"duracao_s":30,"porta":5432}'));
});

test("shows the server refusal and blocks recording without the sniffer", async () => {
  const user = userEvent.setup();
  renderAt("/capturas", (c) =>
    c.method === "POST" ? json(409, { erro: "já há uma captura gravando; espere ela terminar" }) : undefined,
  );
  await screen.findByText("1.200");
  await user.click(screen.getByRole("button", { name: "Gravar captura" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("já há uma captura gravando");

  useAppStore.getState().setEngine("sniffer", "sem-permissao");
  expect(await screen.findByText(/sem permissão de captura/)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Gravar captura" })).toBeDisabled();
});

test("shows progress while a capture records", async () => {
  const recording: Capture = { ...done, id: 9, status: "gravando", fim: undefined, inicio: new Date().toISOString() };
  renderAt("/capturas", (c) => (c.url === "/api/capturas" ? json(200, [recording]) : undefined));
  expect(await screen.findByRole("progressbar", { name: "Progresso da gravação" })).toHaveAttribute("aria-valuemax", "60");
  expect(screen.getByRole("button", { name: "Gravar captura" })).toBeDisabled();
  expect(screen.queryByRole("button", { name: "Apagar a captura 9" })).not.toBeInTheDocument();
});

test("deletes after confirmation", async () => {
  const user = userEvent.setup();
  vi.stubGlobal("confirm", () => true);
  const { calls } = renderAt("/capturas", (c) => (c.method === "DELETE" ? new Response(null, { status: 204 }) : undefined));
  await user.click(await screen.findByRole("button", { name: "Apagar a captura 3" }));
  await waitFor(() => expect(calls.some((c) => c.method === "DELETE" && c.url === "/api/capturas/3")).toBe(true));
});

test("downloads with the token through a Blob", async () => {
  const user = userEvent.setup();
  const create = vi.fn(() => "blob:captura");
  vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: create, revokeObjectURL: vi.fn() }));
  const { calls } = renderAt("/capturas", (c) =>
    c.url === "/api/capturas/3/arquivo" ? new Response(new Uint8Array([0xd4, 0xc3, 0xb2, 0xa1])) : undefined,
  );
  await user.click(await screen.findByRole("button", { name: "Baixar a captura 3" }));
  await waitFor(() => expect(create).toHaveBeenCalled());
  expect(calls.find((c) => c.url === "/api/capturas/3/arquivo")?.headers.get("Authorization")).toBe("Bearer t");
});

test("opens a capture in the inspector, page by page", async () => {
  const user = userEvent.setup();
  const packet = (info: string) => ({
    hora: "2026-11-26T12:00:00.123Z",
    origem: "10.0.0.1",
    destino: "10.0.0.2",
    porta_origem: 40000,
    porta_destino: 3000,
    protocolo: "HTTP",
    tamanho: 100,
    flags: "PSH,ACK",
    info,
    seq: 0,
    ack: 0,
    carga: 0,
  });
  const page = (n: number): CapturePage => ({
    pacotes: [packet(`GET /pagina-${n}`)],
    pagina: n,
    por_pagina: 500,
    total: 501,
  });
  const { calls } = renderAt("/inspetor/captura/3", (c) => {
    const m = /^\/api\/capturas\/3\/pacotes\?pagina=(\d+)$/.exec(c.url);
    return m ? json(200, page(Number(m[1]))) : undefined;
  });
  // O jsdom não mede a área de rolagem, então a tabela virtualizada não
  // desenha as linhas; os números do cabeçalho mostram o que chegou.
  const stat = (label: string) => screen.getByText(label).nextElementSibling;
  await waitFor(() => expect(stat("Pacotes no arquivo")).toHaveTextContent("501"));
  expect(stat("Nesta página")).toHaveTextContent("1");
  expect(stat("Página")).toHaveTextContent("1 de 2");
  expect(screen.getByRole("button", { name: "Anterior" })).toBeDisabled();

  await user.click(screen.getByRole("button", { name: "Próxima" }));
  await waitFor(() => expect(stat("Página")).toHaveTextContent("2 de 2"));
  expect(screen.getByRole("button", { name: "Próxima" })).toBeDisabled();
  expect(calls.map((c) => c.url)).toContain("/api/capturas/3/pacotes?pagina=1");
});

test("formatBytes picks the unit", () => {
  expect(formatBytes(24)).toBe("24 B");
  expect(formatBytes(1536)).toBe("1,5 KiB");
  expect(formatBytes(100 * 1024 * 1024)).toBe("100 MiB");
});
