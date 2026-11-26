import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import type { GitleaksReport } from "../api/reports";
import type { Scan, Tool } from "../api/scans";
import gitleaks from "./__fixtures__/gitleaks.json";
import GitleaksView, { byFile, gitleaksSummary } from "./GitleaksView";
import { missingRequirement, toolBySlug, toolPath } from "./registry";

const scan: Scan = { id: 8, alvo_id: 1, ferramentas: ["gitleaks"], status: "concluida", criada_em: "2026-11-23T12:00:00Z", achados: {} };

test("gitleaks groups the leaks by file, with line, rule and commit", () => {
  render(<GitleaksView report={gitleaks} findings={[]} scan={scan} />);
  const files = within(screen.getByRole("list", { name: "Arquivos" }));
  const aws = files.getByRole("listitem", { name: "config/aws.env" });
  expect(within(aws).getByText("2 vazamentos")).toBeInTheDocument();
  const rows = within(aws).getAllByText(/^linha /).map((r) => r.textContent);
  expect(rows).toEqual(["linha 1", "linha 4"]);
  expect(within(aws).getByText("aws-access-token")).toBeInTheDocument();
  expect(within(aws).getAllByText("2a84462")).toHaveLength(2);
  expect(within(aws).getAllByText(/Dev Cobaia/)).toHaveLength(2);
  expect(screen.getAllByText("valor ocultado")).toHaveLength(3);
  expect(gitleaksSummary(gitleaks)).toBe("3 vazamentos em 2 arquivos");
});

test("gitleaks filters by rule", async () => {
  const user = userEvent.setup();
  render(<GitleaksView report={gitleaks} findings={[]} scan={scan} />);
  await user.click(screen.getByRole("combobox", { name: "Regra" }));
  await user.click(screen.getByRole("option", { name: "github-pat" }));
  expect(screen.getByRole("listitem", { name: "app.py" })).toBeInTheDocument();
  expect(screen.queryByRole("listitem", { name: "config/aws.env" })).toBeNull();
});

test("gitleaks never renders a secret, even if the report carries one", () => {
  const leaked = {
    vazamentos: gitleaks.vazamentos.map((l) => ({ ...l, segredo: "omni-segredo-que-nao-pode-aparecer", trecho: "TOKEN=omni-segredo-que-nao-pode-aparecer" })),
  };
  const { container } = render(<GitleaksView report={leaked} findings={[]} scan={scan} />);
  expect(container.textContent).not.toContain("omni-segredo-que-nao-pode-aparecer");
  expect(container.innerHTML).not.toContain("omni-segredo-que-nao-pode-aparecer");
});

test("gitleaks without git history and without leaks", () => {
  const rep: GitleaksReport = {
    vazamentos: [{ regra: "x", descricao: "", arquivo: "a.env", linha: 2, commit: "", autor: "", data: "", entropia: 3 }],
  };
  const { unmount } = render(<GitleaksView report={rep} findings={[]} scan={scan} />);
  expect(screen.getByText("fora do histórico do git")).toBeInTheDocument();
  expect(gitleaksSummary(rep)).toBe("1 vazamento em 1 arquivo");
  unmount();

  render(<GitleaksView report={{ vazamentos: [] }} findings={[]} scan={scan} />);
  expect(screen.getByText(/não encontrou nenhum segredo/)).toBeInTheDocument();
  expect(gitleaksSummary({ vazamentos: [] })).toBe("Nenhum segredo encontrado");
});

test("byFile sorts files and lines", () => {
  const leak = (arquivo: string, linha: number) => ({ regra: "r", descricao: "", arquivo, linha, commit: "", autor: "", data: "", entropia: 0 });
  expect(byFile([leak("b", 9), leak("a", 3), leak("b", 1)]).map(([f, l]) => [f, l.map((x) => x.linha)])).toEqual([
    ["a", [3]],
    ["b", [1, 9]],
  ]);
});

test("gitleaks lives at /arsenal/vazamentos and needs the source code", () => {
  expect(toolPath("gitleaks")).toBe("/arsenal/vazamentos");
  expect(toolBySlug("vazamentos")).toBe("gitleaks");
  expect(toolBySlug("gitleaks")).toBeNull();
  expect(toolBySlug("amass")).toBe("amass");
  const tool: Tool = { nome: "gitleaks", descricao: "", imagem: "zricethezav/gitleaks:v8.30.0", requer: "repositorio" };
  expect(missingRequirement(tool, { id: 1, nome: "loja", host: "127.0.0.1", portas_app: [3000] })).toBe("Precisa do código-fonte do alvo");
  expect(missingRequirement(tool, { id: 1, nome: "loja", host: "127.0.0.1", portas_app: [3000], repositorio: "/srv/loja" })).toBeNull();
});
