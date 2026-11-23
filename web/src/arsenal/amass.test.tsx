import { render, screen, within } from "@testing-library/react";
import { expect, test } from "vitest";
import type { AmassReport, Subdomain } from "../api/reports";
import type { Scan, Tool } from "../api/scans";
import amass from "./__fixtures__/amass.json";
import AmassView, { amassSummary, tree } from "./AmassView";
import { missingRequirement } from "./registry";

const scan: Scan = { id: 7, alvo_id: 1, ferramentas: ["amass"], status: "concluida", criada_em: "2026-11-23T12:00:00Z", achados: {} };

const sub = (nome: string, fontes: string[] = []): Subdomain => ({ nome, enderecos: [], fontes, neste_servidor: false });

test("amass lists the subdomains under the domain and highlights this server", () => {
  render(<AmassView report={amass} findings={[]} scan={scan} />);
  expect(screen.getByText("Consulta fontes públicas na internet; o alvo não recebe tráfego.")).toBeInTheDocument();
  const scanme = screen.getByRole("listitem", { name: "scanme.nmap.org" });
  expect(within(scanme).getByText("aponta para este servidor")).toBeInTheDocument();
  expect(within(scanme).getByText(/45\.33\.32\.156/)).toBeInTheDocument();
  expect(within(scanme).getByText("via HackerTarget")).toBeInTheDocument();
  const echo = screen.getByRole("listitem", { name: "echo.nmap.org" });
  expect(within(echo).getByText("sem endereço")).toBeInTheDocument();
  expect(within(echo).queryByText("aponta para este servidor")).toBeNull();
  expect(screen.getByText(/4 vieram sem endereço/)).toBeInTheDocument();
  expect(amassSummary(amass)).toBe("6 subdomínios · 1 neste servidor");
});

test("amass tree nests deeper subdomains under their parent label", () => {
  const [root] = tree("loja.test", [sub("b.api.loja.test"), sub("loja.test"), sub("api.loja.test"), sub("admin.loja.test")]);
  expect(root!.sub?.nome).toBe("loja.test");
  expect(root!.children.map((c) => c.label)).toEqual(["admin", "api"]);
  expect(root!.children[1]!.children.map((c) => c.name)).toEqual(["b.api.loja.test"]);
});

test("amass with no addresses at all says it once, not on every row", () => {
  const rep: AmassReport = { dominio: "loja.test", subdominios: [sub("a.loja.test", ["crtsh"]), sub("b.loja.test", ["crtsh"])] };
  render(<AmassView report={rep} findings={[]} scan={scan} />);
  expect(screen.getByText(/Nenhum subdomínio tem endereço/)).toBeInTheDocument();
  expect(screen.queryByText("sem endereço")).toBeNull();
});

test("amass with no subdomains says so", () => {
  const rep: AmassReport = { dominio: "vazio.test", subdominios: [] };
  render(<AmassView report={rep} findings={[]} scan={scan} />);
  expect(screen.getByText(/não encontrou nenhum subdomínio/)).toBeInTheDocument();
  expect(amassSummary(rep)).toBe("Nenhum subdomínio encontrado");
  expect(amassSummary({ dominio: "x.test", subdominios: [sub("a.x.test")] })).toBe("1 subdomínio");
});

test("amass needs the target's domain", () => {
  const tool: Tool = { nome: "amass", descricao: "", imagem: "caffix/amass:v3.23.3", requer: "dominio" };
  expect(missingRequirement(tool, { id: 1, nome: "loja", host: "127.0.0.1", portas_app: [3000] })).toBe("Precisa do domínio do alvo");
  expect(missingRequirement(tool, { id: 1, nome: "loja", host: "127.0.0.1", portas_app: [3000], dominio: "loja.test" })).toBeNull();
});
