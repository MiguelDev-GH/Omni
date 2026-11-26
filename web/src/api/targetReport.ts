import type { SystemMetrics } from "../store/app";
import type { Alert, Level } from "./alerts";
import { apiFetch } from "./client";
import type { ConnSnapshot } from "./connections";
import type { DNSWindow, QueueSnapshot, SMTPWindow } from "./dnsSmtp";
import type { DBWindow } from "./queries";
import type { HTTPWindow } from "./requests";
import type { Finding, Scan } from "./scans";
import type { Target } from "./targets";
import type { TrafficWindow } from "./traffic";

// MAX_REPORT_HOURS acompanha ReportMaxWindow do backend (internal/api).
export const MAX_REPORT_HOURS = 168;

// CheckPoint é uma faixa de verificações de uma porta: fora do ar se qualquer
// verificação da faixa falhou, com a latência média.
export type CheckPoint = {
  alvo_id: number;
  porta: number;
  tipo: string;
  no_ar: boolean;
  status_http?: number;
  latencia_ms: number;
  erro?: string;
  hora: string;
};

export type PortUptime = {
  porta: number;
  tipo: string;
  verificacoes: number;
  no_ar: number;
  percentual: number;
  latencia_media_ms: number;
};

// AlertRuleSummary conta, sem limite, as mudanças de nível de uma regra.
export type AlertRuleSummary = {
  regra: string;
  servico: string;
  nome: string;
  recursos: number;
  mudancas: Record<Level, number>;
};

export type ReportScan = Scan & { lista_achados: Finding[] };

// DBSizeSample é o maior tamanho de um banco numa faixa do relatório.
export type DBSizeSample = { banco: string; bytes: number; hora: string };

// TargetReport é a resposta de GET /api/alvos/{id}/relatorio. Tráfego, HTTP,
// banco, DNS, SMTP, conexões e sistema são do servidor inteiro, não só do
// alvo.
export type TargetReport = {
  alvo: Target;
  desde: string;
  ate: string;
  passo_s: number;
  contagem_alertas: Record<Level, number>;
  resumo_alertas: AlertRuleSummary[];
  // alertas traz até 100 mudanças, as mais graves primeiro.
  alertas: Alert[];
  disponibilidade: PortUptime[];
  verificacoes: CheckPoint[];
  trafego: TrafficWindow[];
  http: HTTPWindow[];
  banco: DBWindow[];
  sistema: SystemMetrics[];
  dns?: DNSWindow[];
  smtp?: SMTPWindow[];
  // fila_smtp e conexoes trazem o pico de cada faixa.
  fila_smtp?: QueueSnapshot[];
  conexoes?: ConnSnapshot[];
  tamanho_banco?: DBSizeSample[];
  varreduras: ReportScan[];
};

// rfc3339 tira os milissegundos, que o backend não aceita.
const rfc3339 = (d: Date) => d.toISOString().replace(/\.\d{3}Z$/, "Z");

export function getTargetReport(id: number, from: Date, to: Date): Promise<TargetReport> {
  const q = `?desde=${encodeURIComponent(rfc3339(from))}&ate=${encodeURIComponent(rfc3339(to))}`;
  return apiFetch<TargetReport>(`/api/alvos/${id}/relatorio${q}`);
}
