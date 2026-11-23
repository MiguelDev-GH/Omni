import { useCallback, useEffect, useRef } from "react";
// O índice do pacote é CJS com "default" em getter, que o build do Vite não
// resolve; o módulo interno exporta a função com nome.
import { useWebSocket } from "react-use-websocket/dist/lib/use-websocket";
import { ApiError, apiFetch } from "../api/client";
import { useAppStore } from "../store/app";
import { handleEvent } from "./events";
import { bindInspectorSocket } from "./inspector";

const MAX_BACKOFF_MS = 30_000;

const defaultInterval = (attempt: number) => Math.min(1000 * 2 ** attempt, MAX_BACKOFF_MS);

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export function buildSocketUrl(loc: Pick<Location, "protocol" | "host">, ticket: string): string {
  const scheme = loc.protocol === "https:" ? "wss:" : "ws:";
  return `${scheme}//${loc.host}/ws?ticket=${encodeURIComponent(ticket)}`;
}

type Options = { reconnectInterval?: (attempt: number) => number };

// useOmniSocket mantém a conexão com o hub. Cada (re)conexão pede um ticket
// novo; falha de rede no ticket é tentada de novo com backoff, e um 401 apaga
// o token (via apiFetch), o que desmonta o layout e encerra as tentativas.
export function useOmniSocket(options: Options = {}): void {
  // A URL assíncrona precisa ser estável: uma função nova reabriria a conexão.
  const intervalRef = useRef(options.reconnectInterval ?? defaultInterval);
  useEffect(() => {
    intervalRef.current = options.reconnectInterval ?? defaultInterval;
  });
  const interval = useCallback((attempt: number) => intervalRef.current(attempt), []);
  const setEngine = useAppStore((s) => s.setEngine);

  const getUrl = useCallback(async () => {
    for (let attempt = 0; ; attempt++) {
      if (!useAppStore.getState().token) throw new Error("sem token");
      try {
        const { ticket } = await apiFetch<{ ticket: string }>("/api/ws/ticket", { method: "POST" });
        return buildSocketUrl(window.location, ticket);
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) throw err;
        await sleep(interval(attempt));
      }
    }
  }, [interval]);

  const { sendMessage } = useWebSocket(getUrl, {
    retryOnError: false,
    shouldReconnect: () => true,
    reconnectAttempts: Infinity,
    reconnectInterval: interval,
    onOpen: () => {
      setEngine("poller", "ativo");
      bindInspectorSocket((msg) => sendRef.current(msg));
    },
    onClose: () => {
      setEngine("poller", "indisponivel");
      bindInspectorSocket(null);
    },
    onMessage: (ev: MessageEvent) => {
      if (typeof ev.data === "string") handleEvent(ev.data);
    },
  });
  // onOpen roda antes de sendMessage ficar disponível neste render; a ref
  // sempre aponta para a versão atual.
  const sendRef = useRef(sendMessage);
  useEffect(() => {
    sendRef.current = sendMessage;
  });
  useEffect(() => () => bindInspectorSocket(null), []);
}
