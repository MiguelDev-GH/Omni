import { apiFetch } from "./client";

export type Level = "verde" | "amarelo" | "vermelho";
export type Service = "web" | "banco" | "sistema";

// Alert é uma mudança de nível, como no evento ALERTA.
export type Alert = {
  id: number;
  regra: string;
  servico: string;
  nome: string;
  recurso: string;
  nivel: Level;
  nivel_anterior: Level;
  valor: number;
  limite: number;
  unidade: string;
  detalhe?: string;
  hora: string;
};

// AlertStatus é o nível atual de uma regra para um recurso.
export type AlertStatus = {
  regra: string;
  servico: string;
  nome: string;
  recurso: string;
  nivel: Level;
  valor: number;
  unidade: string;
  limite_amarelo: number;
  limite_vermelho: number;
  desde: string;
  atualizado: string;
};

export type AlertRule = {
  id: string;
  servico: string;
  nome: string;
  unidade: string;
  limite_amarelo: number;
  limite_vermelho: number;
};

export type AlertsOverview = {
  estado: AlertStatus[];
  historico: Alert[];
  regras: AlertRule[];
  email: { ativo: boolean; destinatarios: number };
};

export function getAlerts(): Promise<AlertsOverview> {
  return apiFetch<AlertsOverview>("/api/alertas");
}

export const statusKey = (regra: string, recurso: string) => `${regra}|${recurso}`;

export const levelRank: Record<Level, number> = { verde: 0, amarelo: 1, vermelho: 2 };

export const levelLabel: Record<Level, string> = { verde: "Verde", amarelo: "Amarelo", vermelho: "Vermelho" };

// Valores da regra de disponibilidade, iguais aos do backend.
export function availabilityText(v: number): string {
  if (v >= 2) return "fora do ar";
  if (v >= 1) return "erro 5xx";
  return "no ar";
}

export function formatAlertValue(regra: string, valor: number, unidade: string): string {
  if (regra === "disponibilidade") return availabilityText(valor);
  const n = Number.isInteger(valor) ? String(valor) : valor.toFixed(1);
  if (unidade === "") return n;
  if (unidade === "%") return `${n}%`;
  return `${n} ${unidade}`;
}
