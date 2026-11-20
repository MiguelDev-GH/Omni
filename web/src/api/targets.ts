import { useAppStore } from "../store/app";
import { apiFetch } from "./client";

export type Target = {
  id: number;
  nome: string;
  host: string;
  portas_app: number[];
  porta_banco?: number;
  dominio?: string;
};

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
