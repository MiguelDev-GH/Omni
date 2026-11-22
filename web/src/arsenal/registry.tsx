import type { Finding, Scan, Tool } from "../api/scans";
import type { Target } from "../api/targets";
import FfufOptions from "./FfufOptions";
import FfufView, { ffufSummary } from "./FfufView";
import NiktoOptions from "./NiktoOptions";
import NiktoView, { niktoSummary } from "./NiktoView";
import NmapView, { nmapSummary } from "./NmapView";
import NucleiView, { nucleiSummary } from "./NucleiView";
import SqlmapOptions from "./SqlmapOptions";
import SqlmapView, { sqlmapSummary } from "./SqlmapView";
import TestsslView, { testsslSummary } from "./TestsslView";

// ToolView liga uma ferramenta do Arsenal à página dela: o título, o resumo
// do último relatório, a visualização e, quando houver, o formulário de opções.
export type ToolView = {
  title: string;
  // tagline é o subtítulo da página, dizendo o que a visualização mostra.
  tagline: string;
  summary: (report: unknown) => string;
  Report: React.FC<{ report: unknown; findings: Finding[]; scan: Scan }>;
  Options?: React.FC<{ value: Record<string, string>; onChange: (v: Record<string, string>) => void; target: Target }>;
  // blocked diz por que a varredura ainda não pode começar com estas opções
  // (opções obrigatórias em falta), ou null.
  blocked?: (options: Record<string, string>) => string | null;
  // pageOnly explica por que a ferramenta só roda na página dela, e não na
  // visão geral: ela precisa de opções que só a página pede.
  pageOnly?: string;
};

export const toolViews: Record<string, ToolView> = {
  nmap: { title: "Nmap", tagline: "Mapa de portas", summary: nmapSummary, Report: NmapView },
  nuclei: { title: "Nuclei", tagline: "O que os templates acharam", summary: nucleiSummary, Report: NucleiView },
  nikto: { title: "Nikto", tagline: "Checagens do servidor", summary: niktoSummary, Report: NiktoView, Options: NiktoOptions },
  sqlmap: {
    title: "SQLMap",
    tagline: "Teste de injeção de SQL",
    summary: sqlmapSummary,
    Report: SqlmapView,
    Options: SqlmapOptions,
    blocked: (o) => (o.rota && o.parametro ? null : "Escolha a rota e informe o parâmetro"),
    pageOnly: "Escolha a rota e o parâmetro na página do SQLMap",
  },
  ffuf: { title: "ffuf", tagline: "Árvore de rotas", summary: ffufSummary, Report: FfufView, Options: FfufOptions },
  testssl: { title: "testssl", tagline: "Boletim TLS", summary: testsslSummary, Report: TestsslView },
};

// missingRequirement diz por que a ferramenta não roda no alvo, ou null.
export function missingRequirement(tool: Tool, target: Target | undefined): string | null {
  if (toolViews[tool.nome]?.pageOnly) return toolViews[tool.nome]!.pageOnly!;
  if (tool.requer === "dominio" && !target?.dominio) return "Precisa do domínio do alvo";
  if (tool.requer === "repositorio" && !target?.repositorio) return "Precisa do código-fonte do alvo";
  return null;
}
