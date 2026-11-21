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
