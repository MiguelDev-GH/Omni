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
