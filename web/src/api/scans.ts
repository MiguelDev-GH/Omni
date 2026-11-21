import { apiFetch } from "./client";

export type Severity = "critica" | "alta" | "media" | "baixa" | "info";

// severities vai da mais grave para a menos grave.
export const severities: Severity[] = ["critica", "alta", "media", "baixa", "info"];

export const severityLabels: Record<Severity, string> = {
  critica: "Crítica",
  alta: "Alta",
  media: "Média",
  baixa: "Baixa",
  info: "Info",
};

export type ScanStatus = "fila" | "rodando" | "concluida" | "falhou";

export type Scan = {
  id: number;
  alvo_id: number;
  ferramentas: string[];
  status: ScanStatus;
  erro?: string;
  criada_em: string;
  inicio?: string;
  fim?: string;
  achados: Partial<Record<Severity, number>>;
};

export type Finding = {
  ferramenta: string;
  titulo: string;
  evidencia: string;
  severidade: Severity;
  porta: number;
  assinatura: string;
};

export type Tool = { nome: string; descricao: string; imagem: string };

export type Comparison = { novos: Finding[]; resolvidos: Finding[]; anteriores: Record<string, number> };

export function listTools(): Promise<Tool[]> {
  return apiFetch<Tool[]>("/api/ferramentas");
}

export function startScan(alvoId: number, ferramentas: string[]): Promise<{ id: number }> {
  return apiFetch<{ id: number }>("/api/varreduras", {
    method: "POST",
    body: JSON.stringify({ alvo_id: alvoId, ferramentas }),
  });
}

export function listScans(alvoId?: number): Promise<Scan[]> {
  return apiFetch<Scan[]>(alvoId ? `/api/varreduras?alvo_id=${alvoId}` : "/api/varreduras");
}

export function scanFindings(id: number): Promise<Finding[]> {
  return apiFetch<Finding[]>(`/api/varreduras/${id}/achados`);
}

export function compareScan(id: number): Promise<Comparison> {
  return apiFetch<Comparison>(`/api/varreduras/${id}/comparacao`);
}

// sortFindings ordena da severidade mais grave para a menos grave, depois por porta e título.
export function sortFindings(list: Finding[]): Finding[] {
  const rank = (s: Severity) => severities.indexOf(s);
  return [...list].sort(
    (a, b) => rank(a.severidade) - rank(b.severidade) || a.porta - b.porta || a.titulo.localeCompare(b.titulo),
  );
}
