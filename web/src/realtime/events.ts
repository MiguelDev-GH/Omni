import { portKey, useAppStore, type PortStatus, type SystemMetrics } from "../store/app";

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
  }
}
