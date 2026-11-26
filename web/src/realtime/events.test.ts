import { expect, test } from "vitest";
import { useAppStore } from "../store/app";
import { handleEvent } from "./events";

const down = { tipo: "ALERTA_QUEDA", dados: { alvo_id: 1, alvo: "api", porta: 3000, erro: "connection refused", hora: "2026-11-20T17:31:52Z" } };
const up = { tipo: "SERVICO_RESTAURADO", dados: { alvo_id: 1, alvo: "api", porta: 3000, hora: "2026-11-20T17:31:56Z" } };
const status = {
  tipo: "STATUS_ALVOS",
  dados: {
    resultados: [
      { alvo_id: 1, porta: 3000, tipo: "http", no_ar: true, status_http: 200, latencia_ms: 1.2, hora: "2026-11-20T17:31:46Z" },
      { alvo_id: 1, porta: 5432, tipo: "tcp", no_ar: false, latencia_ms: 0.4, erro: "recusada", hora: "2026-11-20T17:31:46Z" },
    ],
  },
};

const send = (ev: unknown) => handleEvent(JSON.stringify(ev));

test("ALERTA_QUEDA pushes error toast", () => {
  send(down);
  const st = useAppStore.getState();
  expect(st.toasts).toHaveLength(1);
  expect(st.toasts[0]).toMatchObject({ kind: "erro", title: "Serviço fora do ar", body: "api, porta 3000: connection refused" });
  expect(st.targetStatus["1:3000"]?.no_ar).toBe(false);
});

test("SERVICO_RESTAURADO pushes ok toast", () => {
  send(down);
  send(up);
  const st = useAppStore.getState();
  expect(st.toasts[1]).toMatchObject({ kind: "ok", title: "Serviço restaurado", body: "api, porta 3000" });
  expect(st.targetStatus["1:3000"]?.no_ar).toBe(true);
});

test("STATUS_ALVOS stores each port", () => {
  send(status);
  const ts = useAppStore.getState().targetStatus;
  expect(ts["1:3000"]).toMatchObject({ no_ar: true, status_http: 200 });
  expect(ts["1:5432"]).toMatchObject({ no_ar: false, tipo: "tcp" });
});

test("METRICAS_SISTEMA stores metrics", () => {
  send({ tipo: "METRICAS_SISTEMA", dados: { cpu: 10.9, memoria: 50.2, disco: 4.7, hora: "2026-11-20T17:31:49Z" } });
  expect(useAppStore.getState().system).toEqual({ cpu: 10.9, memoria: 50.2, disco: 4.7, hora: "2026-11-20T17:31:49Z" });
});

test("ignores bad input", () => {
  const before = useAppStore.getState();
  for (const raw of ["não é json", '{"tipo":"OUTRO","dados":{}}', '{"tipo":"ALERTA_QUEDA"}', "null", '{"tipo":"STATUS_ALVOS","dados":{"resultados":5}}']) {
    expect(() => handleEvent(raw)).not.toThrow();
  }
  const after = useAppStore.getState();
  expect(after.toasts).toEqual(before.toasts);
  expect(after.targetStatus).toEqual(before.targetStatus);
  expect(after.system).toBeNull();
});

test("keeps at most five toasts", () => {
  for (let i = 0; i < 6; i++) send({ ...down, dados: { ...down.dados, porta: 3000 + i } });
  const toasts = useAppStore.getState().toasts;
  expect(toasts).toHaveLength(5);
  expect(toasts[0]?.body).toContain("porta 3001");
});

const traffic = {
  tipo: "METRICAS_TRAFEGO",
  dados: {
    inicio: "2026-11-21T12:00:00Z",
    bytes: 1500,
    pacotes: 3,
    descartados: 0,
    protocolos: { HTTP: 1500 },
    top_origens: [{ ip: "10.0.0.1", bytes: 1500, pacotes: 3 }],
  },
};

test("METRICAS_TRAFEGO appends window", () => {
  send(traffic);
  send({ tipo: "METRICAS_TRAFEGO", dados: { inicio: "x" } });
  const list = useAppStore.getState().traffic;
  expect(list).toHaveLength(1);
  expect(list[0]?.bytes).toBe(1500);
});

test("ESTADO_MOTORES sets sniffer state", () => {
  send({ tipo: "ESTADO_MOTORES", dados: { sniffer: "sem-permissao" } });
  expect(useAppStore.getState().engines.sniffer).toBe("sem-permissao");
  send({ tipo: "ESTADO_MOTORES", dados: { sniffer: "quebrado" } });
  expect(useAppStore.getState().engines.sniffer).toBe("sem-permissao");
});

test("METRICAS_HTTP stores the window and ignores malformed ones", () => {
  send({ tipo: "METRICAS_HTTP", dados: { inicio: "2026-11-21T12:00:00Z", requisicoes: 3, status: { "2xx": 3 }, p50_ms: 1, p95_ms: 2, rotas: [] } });
  send({ tipo: "METRICAS_HTTP", dados: { inicio: "2026-11-21T12:00:10Z", requisicoes: "3" } });
  expect(useAppStore.getState().http).toHaveLength(1);
  expect(useAppStore.getState().http[0]?.requisicoes).toBe(3);
});

test("METRICAS_BANCO stores the window", () => {
  send({ tipo: "METRICAS_BANCO", dados: { inicio: "2026-11-21T12:00:00Z", consultas: 7, erros: 1, tipos: { SELECT: 7 }, p50_ms: 1, p95_ms: 2, top: [] } });
  send({ tipo: "METRICAS_BANCO", dados: { inicio: "2026-11-21T12:00:10Z" } });
  expect(useAppStore.getState().db).toHaveLength(1);
});

const alertEvent = (over: Record<string, unknown>) => ({
  tipo: "ALERTA",
  dados: {
    id: 1,
    regra: "http_p95",
    servico: "web",
    nome: "Latência p95 das requisições",
    recurso: "",
    nivel: "amarelo",
    nivel_anterior: "verde",
    valor: 1234.5,
    limite: 1000,
    unidade: "ms",
    hora: "2026-11-21T12:00:00Z",
    ...over,
  },
});

test("ALERTA stores the alert and warns", () => {
  send(alertEvent({}));
  const st = useAppStore.getState();
  expect(st.alertHistory).toHaveLength(1);
  expect(st.alertStatus["http_p95|"]).toMatchObject({ nivel: "amarelo", valor: 1234.5 });
  expect(st.toasts[0]).toMatchObject({ kind: "aviso", title: "Atenção: Latência p95 das requisições", body: "web: 1234.5 ms" });
});

test("ALERTA red is an error toast and green is ok", () => {
  send(alertEvent({ id: 1, nivel: "vermelho" }));
  send(alertEvent({ id: 2, nivel: "verde", nivel_anterior: "vermelho", valor: 10 }));
  const st = useAppStore.getState();
  expect(st.toasts.map((t) => t.kind)).toEqual(["erro", "ok"]);
  expect(st.alertHistory.map((a) => a.id)).toEqual([2, 1]);
  expect(st.alertStatus["http_p95|"]?.nivel).toBe("verde");
});

test("ALERTA for a port going down does not repeat the ALERTA_QUEDA toast", () => {
  send(alertEvent({ regra: "disponibilidade", recurso: "loja:3000", nivel: "vermelho", valor: 2, unidade: "" }));
  const st = useAppStore.getState();
  expect(st.toasts).toHaveLength(0);
  expect(st.alertStatus["disponibilidade|loja:3000"]?.nivel).toBe("vermelho");
});

test("ALERTA with bad data is ignored", () => {
  send(alertEvent({ nivel: "roxo" }));
  send({ tipo: "ALERTA", dados: { id: "1" } });
  expect(useAppStore.getState().alertHistory).toHaveLength(0);
});

test("METRICAS_CONEXOES stores the snapshot and ignores malformed ones", () => {
  send({ tipo: "METRICAS_CONEXOES", dados: { hora: "2026-11-25T12:00:00Z", portas: [{ porta: 3000, tipo: "app", estabelecidas: 2, estados: {}, clientes: [] }] } });
  send({ tipo: "METRICAS_CONEXOES", dados: { hora: "2026-11-25T12:00:05Z" } });
  expect(useAppStore.getState().connections).toHaveLength(1);
  expect(useAppStore.getState().connections[0]?.portas[0]?.estabelecidas).toBe(2);
});

test("METRICAS_DNS, METRICAS_SMTP and FILA_SMTP are stored", () => {
  send({ tipo: "METRICAS_DNS", dados: { inicio: "2026-11-25T12:00:00Z", consultas: 5, falhas: 1, rcodes: { NOERROR: 4 }, p50_ms: 1, p95_ms: 2 } });
  send({ tipo: "METRICAS_SMTP", dados: { inicio: "2026-11-25T12:00:00Z", tentativas: 3, entregues: 3, rejeitados: 0, abortados: 0, codigos: {}, p50_ms: 1, p95_ms: 2 } });
  send({ tipo: "FILA_SMTP", dados: { hora: "2026-11-25T12:00:00Z", mensagens: 9 } });
  send({ tipo: "METRICAS_DNS", dados: { inicio: "2026-11-25T12:00:10Z" } });
  const st = useAppStore.getState();
  expect(st.dns).toHaveLength(1);
  expect(st.smtp[0]?.tentativas).toBe(3);
  expect(st.queue?.mensagens).toBe(9);
});
