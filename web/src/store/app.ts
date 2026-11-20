import { create } from "zustand";
import type { Target } from "../api/targets";
import { readToken, removeToken, writeToken } from "../lib/session";

export type EngineState = "ativo" | "inativo" | "sem-permissao" | "indisponivel";
export type EngineName = "poller" | "sniffer" | "docker";

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
};

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
  // Sniffer e Docker chegam nas Sprints 4 e 7.
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
}));
