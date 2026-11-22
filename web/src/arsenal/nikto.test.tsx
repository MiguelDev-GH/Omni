import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, test } from "vitest";
import type { NiktoReport } from "../api/reports";
import type { Scan } from "../api/scans";
import type { Target } from "../api/targets";
import nikto from "./__fixtures__/nikto.json";
import NiktoOptions from "./NiktoOptions";
import NiktoView, { niktoSummary } from "./NiktoView";

const scan: Scan = { id: 4, alvo_id: 1, ferramentas: ["nikto"], status: "concluida", criada_em: "2026-11-21T12:00:00Z", achados: {} };

test("nikto highlights the server and groups items by category", () => {
  render(<NiktoView report={nikto} findings={[]} scan={scan} />);
  expect(screen.getByLabelText("Servidor identificado")).toHaveTextContent("Loja/0.1");
  const sections = screen.getAllByRole("region");
  expect(sections.map((s) => s.getAttribute("aria-label"))).toEqual(["Arquivos expostos", "Cabeçalhos"]);
  const files = within(sections[0]!);
  expect(files.getByText("Média")).toBeInTheDocument();
  expect(files.getAllByRole("listitem")).toHaveLength(2);
  expect(files.getByText("/.env")).toBeInTheDocument();
  expect(within(sections[1]!).getByText("Baixa")).toBeInTheDocument();
  expect(niktoSummary(nikto)).toBe("Loja/0.1 · 10 itens");
});

test("nikto shows every category in order and only http references as links", () => {
  const rep: NiktoReport = {
    servidor: "",
    itens: [
      { id: "1", metodo: "GET", url: "/", mensagem: "Allowed HTTP Methods: GET.", referencias: [], categoria: "outros" },
      { id: "2", metodo: "GET", url: "/", mensagem: "Apache/2.2 appears to be outdated.", referencias: ["javascript:alert(1)"], categoria: "versao" },
      { id: "3", metodo: "GET", url: "/bk/", mensagem: "Directory indexing found.", referencias: ["https://cwe.example/548"], categoria: "arquivos" },
    ],
  };
  render(<NiktoView report={rep} findings={[]} scan={scan} />);
  expect(screen.getAllByRole("region").map((s) => s.getAttribute("aria-label"))).toEqual([
    "Arquivos expostos",
    "Versão desatualizada",
    "Outros",
  ]);
  expect(screen.getByLabelText("Servidor identificado")).toHaveTextContent("não informado");
  expect(screen.getByRole("link", { name: "https://cwe.example/548" })).toHaveAttribute("rel", "noreferrer");
  expect(screen.queryByRole("link", { name: /javascript/ })).toBeNull();
  expect(niktoSummary(rep)).toBe("3 itens");
  expect(niktoSummary({ servidor: "", itens: [] })).toBe("Nenhum item");
});

test("nikto options pick one of the target's app ports", async () => {
  const target: Target = { id: 1, nome: "loja", host: "127.0.0.1", portas_app: [3000, 8443], porta_banco: 5433 };
  let last: Record<string, string> = {};
  function Harness() {
    const [v, setV] = useState<Record<string, string>>({});
    return <NiktoOptions value={v} onChange={(x) => setV((last = x))} target={target} />;
  }
  render(<Harness />);
  await userEvent.click(screen.getByRole("combobox", { name: "Porta" }));
  await userEvent.click(screen.getByRole("option", { name: "8443" }));
  expect(last).toEqual({ porta: "8443" });
});
