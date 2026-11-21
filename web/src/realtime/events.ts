import type { HTTPWindow } from "../api/requests";
import type { TrafficWindow } from "../api/traffic";
import { portKey, useAppStore, type EngineState, type PortStatus, type SystemMetrics } from "../store/app";

type Transition = { alvo_id: number; alvo: string; porta: number; erro?: string; hora: string };

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

function isPortStatus(v: unknown): v is PortStatus {
  return isObject(v) && typeof v.alvo_id === "number" && typeof v.porta === "number" && typeof v.no_ar === "boolean";
}

function isTransition(v: unknown): v is Transition {
  return isObject(v) && typeof v.alvo_id === "number" && typeof v.alvo === "string" && typeof v.porta === "number";
}

function isMetrics(v: unknown): v is SystemMetrics {
  return isObject(v) && ["cpu", "memoria", "disco"].every((k) => typeof v[k] === "number");
}

function isTrafficWindow(v: unknown): v is TrafficWindow {
  return (
    isObject(v) &&
    typeof v.inicio === "string" &&
    typeof v.bytes === "number" &&
    typeof v.pacotes === "number" &&
    isObject(v.protocolos) &&
    Array.isArray(v.top_origens)
  );
}

function isHTTPWindow(v: unknown): v is HTTPWindow {
  return isObject(v) && typeof v.inicio === "string" && typeof v.requisicoes === "number" && isObject(v.status);
}

const engineStates: EngineState[] = ["ativo", "inativo", "sem-permissao", "indisponivel"];

const isEngineState = (v: unknown): v is EngineState => engineStates.includes(v as EngineState);

// markPort atualiza só o campo no_ar de uma porta, preservando o resto.
function markPort(t: Transition, up: boolean) {
  const st = useAppStore.getState();
  const prev = st.targetStatus[portKey(t.alvo_id, t.porta)];
  st.setPortStatus([
    { tipo: "http", latencia_ms: 0, ...prev, alvo_id: t.alvo_id, porta: t.porta, no_ar: up, hora: t.hora },
  ]);
}

// handleEvent aplica uma mensagem do WebSocket ao store. Mensagens
// malformadas ou de tipo desconhecido são ignoradas.
export function handleEvent(raw: string): void {
  let ev: unknown;
  try {
    ev = JSON.parse(raw);
  } catch {
    return;
  }
  if (!isObject(ev)) return;
  const st = useAppStore.getState();
  const dados = ev.dados;

  switch (ev.tipo) {
    case "STATUS_ALVOS":
      if (isObject(dados) && Array.isArray(dados.resultados)) {
        st.setPortStatus(dados.resultados.filter(isPortStatus));
      }
      break;
    case "ALERTA_QUEDA":
      if (isTransition(dados)) {
        markPort(dados, false);
        st.pushToast({ kind: "erro", title: "Serviço fora do ar", body: `${dados.alvo}, porta ${dados.porta}: ${dados.erro ?? ""}` });
      }
      break;
    case "SERVICO_RESTAURADO":
      if (isTransition(dados)) {
        markPort(dados, true);
        st.pushToast({ kind: "ok", title: "Serviço restaurado", body: `${dados.alvo}, porta ${dados.porta}` });
      }
      break;
    case "METRICAS_SISTEMA":
      if (isMetrics(dados)) st.setSystem(dados);
      break;
    case "METRICAS_TRAFEGO":
      if (isTrafficWindow(dados)) st.addTraffic([dados]);
      break;
    case "METRICAS_HTTP":
      if (isHTTPWindow(dados)) st.addHTTP([dados]);
      break;
    case "ESTADO_MOTORES":
      if (isObject(dados) && isEngineState(dados.sniffer)) st.setEngine("sniffer", dados.sniffer);
      break;
  }
}
