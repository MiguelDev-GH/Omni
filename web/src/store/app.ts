import { create } from "zustand";
import { statusKey, type Alert, type AlertStatus } from "../api/alerts";
import type { ConnSnapshot } from "../api/connections";
import type { DNSWindow, QueueSnapshot, SMTPWindow } from "../api/dnsSmtp";
import type { Target } from "../api/targets";
import type { DBWindow } from "../api/queries";
import type { HTTPWindow } from "../api/requests";
import type { TrafficWindow } from "../api/traffic";
import { mergeConnections } from "../lib/connections";
import { mergeL7 } from "../lib/l7";
import { mergeWindows } from "../lib/traffic";
import { applyTema, initialTema, saveTema, type Tema } from "../lib/tema";
import { readToken, removeToken, writeToken } from "../lib/session";

export type EngineState = "ativo" | "inativo" | "sem-permissao" | "indisponivel";
export type EngineName = "poller" | "sniffer" | "docker";

export type PortStatus = {
  alvo_id: number;
  porta: number;
  tipo: "http" | "tcp" | "dns" | "smtp";
  no_ar: boolean;
  status_http?: number;
  latencia_ms: number;
  erro?: string;
  hora: string;
  degradado?: boolean; // DNS com RCODE de erro ou SMTP com comando recusado
  detalhe?: string;
  starttls?: boolean;
};

export type SystemMetrics = {
  cpu: number;
  memoria: number;
  disco: number;
  hora: string;
  // I/O do disco; os campos só valem com io true.
  io?: boolean;
  leitura_bps?: number;
  escrita_bps?: number;
  iops?: number;
  latencia_disco_ms?: number;
  disco_ocupado?: number;
};

export type Toast = { id: number; kind: "erro" | "aviso" | "ok"; title: string; body: string; link?: { to: string; label: string } };

// MAX_LOG_LINES limita as linhas guardadas do console de cada varredura.
export const MAX_LOG_LINES = 5000;

const MAX_TOASTS = 5;

// MAX_ALERT_HISTORY acompanha o limite do histórico guardado pelo backend.
export const MAX_ALERT_HISTORY = 200;

export const portKey = (alvoId: number, porta: number) => `${alvoId}:${porta}`;

export type AppState = {
  tema: Tema;
  setTema: (t: Tema) => void;
  token: string | null;
  sessionExpired: boolean;
  setToken: (t: string) => void;
  clearToken: (expired: boolean) => void;
  engines: Record<EngineName, EngineState>;
  setEngine: (name: EngineName, s: EngineState) => void;
  targets: Target[];
  targetsLoaded: boolean;
  setTargets: (list: Target[]) => void; // mantém a seleção válida
  selectedTargetId: number | null;
  selectTarget: (id: number | null) => void;
  targetStatus: Record<string, PortStatus>; // chave portKey(alvo_id, porta)
  setPortStatus: (list: PortStatus[]) => void;
  system: SystemMetrics | null;
  setSystem: (m: SystemMetrics) => void;
  traffic: TrafficWindow[]; // cronológico, no máximo MAX_POINTS
  addTraffic: (list: TrafficWindow[]) => void;
  http: HTTPWindow[]; // cronológico, no máximo L7_POINTS
  addHTTP: (list: HTTPWindow[]) => void;
  db: DBWindow[]; // cronológico, no máximo L7_POINTS
  addDB: (list: DBWindow[]) => void;
  connections: ConnSnapshot[]; // cronológico, no máximo MAX_CONN_POINTS
  dns: DNSWindow[]; // cronológico, no máximo L7_POINTS
  addDNS: (list: DNSWindow[]) => void;
  smtp: SMTPWindow[]; // cronológico, no máximo L7_POINTS
  addSMTP: (list: SMTPWindow[]) => void;
  queue: QueueSnapshot | null; // última contagem da fila do SMTP
  setQueue: (q: QueueSnapshot) => void;
  addConnections: (list: ConnSnapshot[]) => void;
  scanLogs: Record<number, string[]>; // por id de varredura
  appendScanLog: (scanId: number, lines: string[]) => void;
  // scanEvents conta os eventos de fim de varredura, para as telas recarregarem.
  scanEvents: number;
  bumpScanEvents: () => void;
  toasts: Toast[];
  pushToast: (t: Omit<Toast, "id">) => void;
  dismissToast: (id: number) => void;
  alertStatus: Record<string, AlertStatus>; // chave statusKey(regra, recurso)
  alertHistory: Alert[]; // do mais novo para o mais antigo
  alertsLoaded: boolean;
  setAlertsOverview: (estado: AlertStatus[], historico: Alert[]) => void;
  addAlert: (a: Alert) => void;
};

let nextToastId = 1;

export const useAppStore = create<AppState>()((set) => ({
  tema: initialTema(),
  setTema: (t) => {
    applyTema(t);
    saveTema(t);
    set({ tema: t });
  },
  token: readToken(),
  sessionExpired: false,
  setToken: (t) => {
    writeToken(t);
    set({ token: t, sessionExpired: false });
  },
  clearToken: (expired) => {
    removeToken();
    set({ token: null, sessionExpired: expired });
  },
  // O sniffer e o Docker informam o estado por ESTADO_MOTORES.
  engines: { poller: "indisponivel", sniffer: "inativo", docker: "inativo" },
  setEngine: (name, s) => set((st) => ({ engines: { ...st.engines, [name]: s } })),
  targets: [],
  targetsLoaded: false,
  setTargets: (list) =>
    set((st) => ({
      targets: list,
      targetsLoaded: true,
      selectedTargetId: list.some((t) => t.id === st.selectedTargetId) ? st.selectedTargetId : (list[0]?.id ?? null),
    })),
  selectedTargetId: null,
  selectTarget: (id) => set({ selectedTargetId: id }),
  targetStatus: {},
  setPortStatus: (list) =>
    set((st) => {
      const next = { ...st.targetStatus };
      for (const p of list) next[portKey(p.alvo_id, p.porta)] = p;
      return { targetStatus: next };
    }),
  system: null,
  setSystem: (m) => set({ system: m }),
  traffic: [],
  addTraffic: (list) => set((st) => ({ traffic: mergeWindows(st.traffic, list) })),
  http: [],
  addHTTP: (list) => set((st) => ({ http: mergeL7(st.http, list) })),
  db: [],
  addDB: (list) => set((st) => ({ db: mergeL7(st.db, list) })),
  connections: [],
  dns: [],
  addDNS: (list) => set((st) => ({ dns: mergeL7(st.dns, list) })),
  smtp: [],
  addSMTP: (list) => set((st) => ({ smtp: mergeL7(st.smtp, list) })),
  queue: null,
  setQueue: (q) => set({ queue: q }),
  addConnections: (list) => set((st) => ({ connections: mergeConnections(st.connections, list) })),
  scanLogs: {},
  appendScanLog: (scanId, lines) =>
    set((st) => ({ scanLogs: { ...st.scanLogs, [scanId]: [...(st.scanLogs[scanId] ?? []), ...lines].slice(-MAX_LOG_LINES) } })),
  scanEvents: 0,
  bumpScanEvents: () => set((st) => ({ scanEvents: st.scanEvents + 1 })),
  toasts: [],
  pushToast: (t) => set((st) => ({ toasts: [...st.toasts, { ...t, id: nextToastId++ }].slice(-MAX_TOASTS) })),
  dismissToast: (id) => set((st) => ({ toasts: st.toasts.filter((t) => t.id !== id) })),
  alertStatus: {},
  alertHistory: [],
  alertsLoaded: false,
  // O estado da API substitui o local; o histórico junta os dois, porque um
  // ALERTA pode chegar pelo WebSocket enquanto a requisição está no caminho.
  setAlertsOverview: (estado, historico) =>
    set((st) => {
      const next: Record<string, AlertStatus> = {};
      for (const s of estado) next[statusKey(s.regra, s.recurso)] = s;
      return { alertStatus: next, alertHistory: mergeAlerts(st.alertHistory, historico), alertsLoaded: true };
    }),
  addAlert: (a) =>
    set((st) => {
      const key = statusKey(a.regra, a.recurso);
      const prev = st.alertStatus[key];
      const status: AlertStatus = {
        limite_amarelo: 0,
        limite_vermelho: 0,
        ...prev,
        regra: a.regra,
        servico: a.servico,
        nome: a.nome,
        recurso: a.recurso,
        nivel: a.nivel,
        valor: a.valor,
        unidade: a.unidade,
        desde: a.hora,
        atualizado: a.hora,
      };
      return { alertStatus: { ...st.alertStatus, [key]: status }, alertHistory: mergeAlerts(st.alertHistory, [a]) };
    }),
}));

// mergeAlerts junta listas de alertas sem repetir ids, do mais novo para o mais antigo.
function mergeAlerts(a: Alert[], b: Alert[]): Alert[] {
  const byId = new Map<number, Alert>();
  for (const x of [...a, ...b]) byId.set(x.id, x);
  return [...byId.values()].sort((x, y) => y.id - x.id).slice(0, MAX_ALERT_HISTORY);
}
