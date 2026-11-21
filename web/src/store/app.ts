import { create } from "zustand";
import type { Target } from "../api/targets";
import type { TrafficWindow } from "../api/traffic";
import { mergeWindows } from "../lib/traffic";
import { readToken, removeToken, writeToken } from "../lib/session";

export type EngineState = "ativo" | "inativo" | "sem-permissao" | "indisponivel";
export type EngineName = "poller" | "sniffer" | "docker";

export type PortStatus = {
  alvo_id: number;
  porta: number;
  tipo: "http" | "tcp";
  no_ar: boolean;
  status_http?: number;
  latencia_ms: number;
  erro?: string;
  hora: string;
};

export type SystemMetrics = { cpu: number; memoria: number; disco: number; hora: string };

export type Toast = { id: number; kind: "erro" | "ok"; title: string; body: string };

const MAX_TOASTS = 5;

export const portKey = (alvoId: number, porta: number) => `${alvoId}:${porta}`;

export type AppState = {
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
  toasts: Toast[];
  pushToast: (t: Omit<Toast, "id">) => void;
  dismissToast: (id: number) => void;
};

let nextToastId = 1;

export const useAppStore = create<AppState>()((set) => ({
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
  // O sniffer informa o estado por ESTADO_MOTORES; o Docker chega na Sprint 7.
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
  toasts: [],
  pushToast: (t) => set((st) => ({ toasts: [...st.toasts, { ...t, id: nextToastId++ }].slice(-MAX_TOASTS) })),
  dismissToast: (id) => set((st) => ({ toasts: st.toasts.filter((t) => t.id !== id) })),
}));
