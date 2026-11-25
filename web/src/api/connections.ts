import { apiFetch } from "./client";
import { sinceQuery } from "./traffic";

export type ConnKind = "app" | "banco" | "smtp";

// ClientPool é o pool de conexões de um processo cliente; processo vazio
// quando o Omni não tem permissão para ver o processo.
export type ClientPool = { processo: string; conexoes: number };

export type PortConns = {
  porta: number;
  tipo: ConnKind;
  estabelecidas: number;
  estados: Record<string, number>; // "ESTABLISHED", "TIME_WAIT"...
  clientes: ClientPool[];
};

// ConnSnapshot é uma coleta de conexões, como em METRICAS_CONEXOES (a cada 5 s).
export type ConnSnapshot = { hora: string; portas: PortConns[] };

export function listConnections(since?: Date): Promise<ConnSnapshot[]> {
  return apiFetch<ConnSnapshot[]>(`/api/conexoes${sinceQuery(since)}`);
}
