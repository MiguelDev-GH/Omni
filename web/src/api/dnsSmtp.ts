import { apiFetch } from "./client";
import { sinceQuery } from "./traffic";

// DNSWindow resume 10 s de consultas DNS vistas no tráfego, como em METRICAS_DNS.
export type DNSWindow = {
  inicio: string;
  consultas: number;
  falhas: number;
  rcodes: Record<string, number>; // "NOERROR", "NXDOMAIN", "TIMEOUT"...
  p50_ms: number;
  p95_ms: number;
};

// SMTPWindow resume 10 s de transações SMTP vistas no tráfego, como em METRICAS_SMTP.
export type SMTPWindow = {
  inicio: string;
  tentativas: number;
  entregues: number;
  rejeitados: number;
  abortados: number;
  codigos: Record<string, number>; // recusas 4xx e 5xx
  p50_ms: number;
  p95_ms: number;
};

// QueueSnapshot é uma contagem da fila de saída, como em FILA_SMTP.
export type QueueSnapshot = { hora: string; mensagens: number; erro?: string };

export type QueueInfo = { configurado: boolean; contagens: QueueSnapshot[] };

export function listDNS(since?: Date): Promise<DNSWindow[]> {
  return apiFetch<DNSWindow[]>(`/api/dns${sinceQuery(since)}`);
}

export function listSMTP(since?: Date): Promise<SMTPWindow[]> {
  return apiFetch<SMTPWindow[]>(`/api/smtp${sinceQuery(since)}`);
}

export function getQueue(): Promise<QueueInfo> {
  return apiFetch<QueueInfo>("/api/smtp/fila");
}
