import { create } from "zustand";
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
}));
