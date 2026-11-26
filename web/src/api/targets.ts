import { useAppStore, type PortStatus } from "../store/app";
import { apiFetch } from "./client";
import { sinceQuery } from "./traffic";

export type Target = {
  id: number;
  nome: string;
  host: string;
  portas_app: number[];
  porta_banco?: number;
  dominio?: string;
  porta_dns?: number;
  porta_smtp?: number;
  // repositorio é o código-fonte do alvo, usado pelo Gitleaks (Sprint 13).
  repositorio?: string;
};

// targetPorts lista todas as portas verificadas de um alvo.
export function targetPorts(t: Target): number[] {
  return [...t.portas_app, ...[t.porta_banco, t.porta_dns, t.porta_smtp].filter((p): p is number => p !== undefined && p > 0)];
}

export type TargetInput = Omit<Target, "id">;

export function listTargets(): Promise<Target[]> {
  return apiFetch<Target[]>("/api/alvos");
}

export function createTarget(t: TargetInput): Promise<Target> {
  return apiFetch<Target>("/api/alvos", { method: "POST", body: JSON.stringify(t) });
}

export function updateTarget(id: number, t: TargetInput): Promise<Target> {
  return apiFetch<Target>(`/api/alvos/${id}`, { method: "PUT", body: JSON.stringify(t) });
}

export function deleteTarget(id: number): Promise<void> {
  return apiFetch<void>(`/api/alvos/${id}`, { method: "DELETE" });
}

// loadTargets busca a lista e a entrega ao store.
export async function loadTargets(): Promise<void> {
  useAppStore.getState().setTargets(await listTargets());
}

// parsePorts aceita vírgulas e espaços como separadores e ignora vazios.
// Devolve null se algum pedaço não for um inteiro; o intervalo fica com o servidor.
export function parsePorts(text: string): number[] | null {
  const parts = text.split(/[\s,]+/).filter((p) => p !== "");
  if (!parts.every((p) => /^\d+$/.test(p))) return null;
  return parts.map(Number);
}

// listHistory busca as verificações das portas do alvo desde since (padrão
// da API: últimas 24 h), em ordem cronológica.
export function listHistory(id: number, since?: Date): Promise<PortStatus[]> {
  return apiFetch<PortStatus[]>(`/api/alvos/${id}/historico${sinceQuery(since)}`);
}
