import { ApiError, apiFetch } from "./client";
import type { Severity } from "./scans";

// Relatórios estruturados de cada ferramenta, como o backend grava
// (internal/arsenal/reports.go).

export type NmapPort = {
  porta: number;
  protocolo: string;
  estado: "open" | "closed" | "filtered" | string;
  servico: string;
  produto: string;
  versao: string;
  extra: string;
};
export type NmapReport = { hosts: { endereco: string; portas: NmapPort[] }[] };

export type NucleiResult = {
  template: string;
  nome: string;
  severidade: Severity;
  tags: string[];
  url: string;
  matcher: string;
  extraidos: string[];
  descricao: string;
  referencias: string[];
};
export type NucleiReport = { resultados: NucleiResult[] };

export type TestsslReport = {
  nota: string;
  protocolos: { nome: string; oferecido: boolean }[];
  certificado: { assunto: string; emissor: string; valido_ate: string | null; dias_restantes: number; confiavel: boolean };
  vulnerabilidades: { id: string; nome: string; vulneravel: boolean; cve: string }[];
};

export type FfufResult = {
  caminho: string;
  status: number;
  tamanho: number;
  palavras: number;
  redirecionamento: string;
  duracao_ms: number;
};
export type FfufReport = { base_url: string; resultados: FfufResult[] };

export type NiktoCategory = "cabecalhos" | "arquivos" | "versao" | "outros";
export type NiktoItem = {
  id: string;
  metodo: string;
  url: string;
  mensagem: string;
  referencias: string[];
  categoria: NiktoCategory;
};
export type NiktoReport = { servidor: string; itens: NiktoItem[] };

export type SqlmapTechnique = { tipo: string; titulo: string; payload: string };
export type SqlmapReport = { url: string; parametro: string; injetavel: boolean; sgbd: string; tecnicas: SqlmapTechnique[] };

// getReport busca o relatório de uma ferramenta na varredura; sem relatório
// (varredura antiga ou falha ao montá-lo), devolve null.
export async function getReport(scanId: number, tool: string): Promise<unknown | null> {
  try {
    return await apiFetch<unknown>(`/api/varreduras/${scanId}/relatorio/${encodeURIComponent(tool)}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return null;
    throw err;
  }
}
