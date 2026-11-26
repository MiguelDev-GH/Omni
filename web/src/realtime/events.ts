import { formatAlertValue, type Alert, type Level } from "../api/alerts";
import type { ConnSnapshot } from "../api/connections";
import type { DBWindow } from "../api/queries";
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

function isDBWindow(v: unknown): v is DBWindow {
  return isObject(v) && typeof v.inicio === "string" && typeof v.consultas === "number" && typeof v.erros === "number";
}

function isConnSnapshot(v: unknown): v is ConnSnapshot {
  return (
    isObject(v) &&
    typeof v.hora === "string" &&
    Array.isArray(v.portas) &&
    v.portas.every((p) => isObject(p) && typeof p.porta === "number" && typeof p.estabelecidas === "number" && Array.isArray(p.clientes))
  );
}

const levels: Level[] = ["verde", "amarelo", "vermelho"];

function isAlert(v: unknown): v is Alert {
  return (
    isObject(v) &&
    typeof v.id === "number" &&
    typeof v.regra === "string" &&
    typeof v.nome === "string" &&
    typeof v.recurso === "string" &&
    typeof v.servico === "string" &&
    typeof v.valor === "number" &&
    levels.includes(v.nivel as Level) &&
    levels.includes(v.nivel_anterior as Level)
  );
}

const alertToastTitle: Record<Level, string> = { verde: "Normalizado", amarelo: "Atenção", vermelho: "Crítico" };
const alertToastKind = { verde: "ok", amarelo: "aviso", vermelho: "erro" } as const;

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
    case "METRICAS_BANCO":
      if (isDBWindow(dados)) st.addDB([dados]);
      break;
    case "METRICAS_CONEXOES":
      if (isConnSnapshot(dados)) st.addConnections([dados]);
      break;
    case "ESTADO_MOTORES":
      if (isObject(dados) && isEngineState(dados.sniffer)) st.setEngine("sniffer", dados.sniffer);
      if (isObject(dados) && isEngineState(dados.docker)) st.setEngine("docker", dados.docker);
      break;
    case "SCAN_LOG":
      if (isObject(dados) && typeof dados.scan_id === "number" && Array.isArray(dados.linhas)) {
        st.appendScanLog(dados.scan_id, dados.linhas.filter((l): l is string => typeof l === "string"));
      }
      break;
    case "SCAN_CONCLUIDO":
      if (isObject(dados) && typeof dados.scan_id === "number") {
        const n = typeof dados.achados === "number" ? dados.achados : 0;
        const novos = typeof dados.novos_desde_ultimo === "number" ? dados.novos_desde_ultimo : 0;
        st.bumpScanEvents();
        st.pushToast({
          kind: "ok",
          title: "Varredura concluída",
          body: `Varredura #${dados.scan_id}: ${n} ${n === 1 ? "achado" : "achados"}, ${novos} ${novos === 1 ? "novo" : "novos"} desde a anterior.`,
          link: { to: `/relatorios?varredura=${dados.scan_id}`, label: "Ver achados" },
        });
      }
      break;
    case "SCAN_FALHOU":
      if (isObject(dados) && typeof dados.scan_id === "number") {
        st.bumpScanEvents();
        st.pushToast({ kind: "erro", title: "Varredura falhou", body: `Varredura #${dados.scan_id}: ${typeof dados.erro === "string" ? dados.erro : ""}` });
      }
      break;
    case "ALERTA":
      if (isAlert(dados)) {
        st.addAlert(dados);
        // Queda e volta de uma porta já têm aviso próprio (ALERTA_QUEDA e
        // SERVICO_RESTAURADO); aqui só entra o "no ar com erro 5xx".
        const duplicate = dados.regra === "disponibilidade" && dados.nivel !== "amarelo" && dados.nivel_anterior !== "amarelo";
        if (duplicate) break;
        const where = dados.recurso || dados.servico;
        const value = formatAlertValue(dados.regra, dados.valor, typeof dados.unidade === "string" ? dados.unidade : "");
        st.pushToast({
          kind: alertToastKind[dados.nivel],
          title: `${alertToastTitle[dados.nivel]}: ${dados.nome}`,
          body: `${where}: ${value}`,
          link: { to: "/alertas", label: "Ver alertas" },
        });
      }
      break;
    case "SCAN_INICIADO":
      if (isObject(dados) && typeof dados.scan_id === "number") st.bumpScanEvents();
      break;
  }
}
