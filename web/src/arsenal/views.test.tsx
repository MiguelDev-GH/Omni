import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import type { NucleiReport, TestsslReport } from "../api/reports";
import type { Scan } from "../api/scans";
import { json, mockFetch } from "../test/fetch";
import nmap from "./__fixtures__/nmap.json";
import nuclei from "./__fixtures__/nuclei.json";
import testssl from "./__fixtures__/testssl.json";
import NmapView, { nmapSummary } from "./NmapView";
import NucleiView, { nucleiSummary } from "./NucleiView";
import TestsslView, { testsslSummary } from "./TestsslView";

const scan: Scan = { id: 7, alvo_id: 1, ferramentas: ["nmap"], status: "concluida", criada_em: "2026-11-21T12:00:00Z", achados: {} };

test("nmap shows every port with a readable state and the changes since the previous scan", async () => {
  mockFetch((c) =>
    c.url === "/api/varreduras/7/comparacao"
      ? json(200, {
          novos: [{ ferramenta: "nmap", titulo: "Porta 55432/tcp aberta", evidencia: "", severidade: "info", porta: 55432, assinatura: "n" }],
          resolvidos: [],
          anteriores: { nmap: 3 },
        })
      : json(500, {}),
  );
  render(<NmapView report={nmap} findings={[]} scan={scan} />);
  const table = screen.getByRole("table", { name: "Mapa de portas" });
  const rows = within(table).getAllByRole("row");
  expect(rows).toHaveLength(4);
  expect(rows[1]).toHaveTextContent("38080/tcp");
  expect(rows[1]).toHaveTextContent("Aberta");
  expect(rows[1]).toHaveTextContent("SimpleHTTPServer 0.6");
  expect(rows[2]).toHaveTextContent("Fechada");
  expect(await within(rows[3]!).findByText("abriu desde a anterior")).toBeInTheDocument();
  expect(within(screen.getByRole("list", { name: "Faixa das portas" })).getAllByRole("listitem")).toHaveLength(3);
  expect(nmapSummary(nmap)).toBe("2 portas abertas de 3");
});

test("nuclei groups results by template and filters by severity and tag", async () => {
  render(<NucleiView report={nuclei} findings={[]} scan={scan} />);
  const groups = within(screen.getByRole("list", { name: "Templates" })).getAllByRole("group");
  expect(groups.length).toBe(3);
  // Mais grave primeiro.
  expect(groups[0]).toHaveTextContent("Git Configuration - Detect");
  expect(groups[0]).toHaveTextContent("Média");

  await userEvent.click(screen.getByRole("checkbox", { name: /Info/ }));
  expect(within(screen.getByRole("list", { name: "Templates" })).getAllByRole("group")).toHaveLength(1);
  await userEvent.click(screen.getByRole("checkbox", { name: /Info/ }));

  await userEvent.click(screen.getByRole("combobox", { name: "Tag" }));
  await userEvent.click(screen.getByRole("option", { name: "tech" }));
  const shown = within(screen.getByRole("list", { name: "Templates" })).getAllByRole("group");
  expect(shown).toHaveLength(1);
  expect(shown[0]).toHaveTextContent("Wappalyzer Technology Detection");
  expect(shown[0]).toHaveTextContent("2 ocorrências");
  expect(nucleiSummary(nuclei)).toMatch(/resultados em 3 templates$/);
});

test("nuclei references open in a new tab and only http links are clickable", () => {
  const rep: NucleiReport = {
    resultados: [
      { template: "x", nome: "X", severidade: "alta", tags: [], url: "http://a/", matcher: "", extraidos: [], descricao: "", referencias: ["https://ref.example/x", "javascript:alert(1)"] },
    ],
  };
  render(<NucleiView report={rep} findings={[]} scan={scan} />);
  const link = screen.getByRole("link", { name: "https://ref.example/x" });
  expect(link).toHaveAttribute("target", "_blank");
  expect(link).toHaveAttribute("rel", "noreferrer");
  expect(screen.queryByRole("link", { name: /javascript/ })).toBeNull();
});

test("testssl shows the grade, protocols, certificate validity and vulnerabilities", () => {
  render(<TestsslView report={testssl} findings={[]} scan={scan} />);
  expect(screen.getByLabelText("Nota geral")).toHaveTextContent("T");
  const protocols = within(screen.getByRole("list", { name: "Protocolos" })).getAllByRole("listitem");
  expect(protocols).toHaveLength(6);
  expect(protocols[5]).toHaveTextContent("TLS 1.3Oferecido");
  expect(protocols[0]).toHaveTextContent("Não oferecido");
  expect(screen.getByText(/2 dias restantes/)).toHaveClass("text-erro");
  expect(screen.getByRole("meter", { name: "Validade do certificado" })).toHaveAttribute("aria-valuenow", "2");
  expect(screen.getByText("Cadeia não confiável")).toBeInTheDocument();
  expect(screen.getByText(/1 encontrada/)).toBeInTheDocument();
  const vulns = within(screen.getByRole("list", { name: "Vulnerabilidades testadas" }));
  expect(vulns.getByText("Lucky13").closest("li")).toHaveTextContent("Vulnerável");
  expect(vulns.getByText("Heartbleed").closest("li")).toHaveTextContent("Não vulnerável");
  expect(testsslSummary(testssl)).toBe("Nota T · certificado vence em 2 dias");
});

test("testssl flags insecure protocols on offer and a missing grade", () => {
  const rep = structuredClone(testssl) as TestsslReport;
  rep.nota = "";
  rep.protocolos[2]!.oferecido = true;
  render(<TestsslView report={rep} findings={[]} scan={scan} />);
  expect(screen.getByLabelText("Nota geral")).toHaveTextContent("—");
  expect(within(screen.getByRole("list", { name: "Protocolos" })).getAllByRole("listitem")[2]).toHaveTextContent(
    "Oferecido: inseguro",
  );
});
