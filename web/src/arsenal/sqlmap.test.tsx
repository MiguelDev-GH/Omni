import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, test } from "vitest";
import type { Scan } from "../api/scans";
import type { Target } from "../api/targets";
import { json, mockFetch } from "../test/fetch";
import sqlmap from "./__fixtures__/sqlmap.json";
import { toolViews } from "./registry";
import SqlmapOptions from "./SqlmapOptions";
import SqlmapView, { sqlmapSummary } from "./SqlmapView";

const scan: Scan = { id: 9, alvo_id: 1, ferramentas: ["sqlmap"], status: "concluida", criada_em: "2026-11-21T12:00:00Z", achados: {} };
const target: Target = { id: 1, nome: "loja", host: "127.0.0.1", portas_app: [3000, 8443], porta_banco: 5433 };
const route = (metodo: string, rota: string) => ({ metodo, rota, requisicoes: 1, erros: 0, p50_ms: 1, p95_ms: 1 });

function Harness({ onValue }: { onValue: (v: Record<string, string>) => void }) {
  const [v, setV] = useState<Record<string, string>>({});
  return (
    <SqlmapOptions
      value={v}
      onChange={(x) => {
        setV(x);
        onValue(x);
      }}
      target={target}
    />
  );
}

test("sqlmap view shows the techniques of an injectable parameter", () => {
  render(<SqlmapView report={sqlmap} findings={[]} scan={scan} />);
  expect(screen.getByLabelText("Veredito")).toHaveTextContent("Injetável");
  expect(screen.getByText("PostgreSQL")).toBeInTheDocument();
  expect(screen.getByText("q=1 AND 5871=5871")).toHaveClass("font-mono");
  expect(screen.getAllByRole("listitem")).toHaveLength(2);
  expect(screen.getByText(/nível 1 e risco 1/)).toBeInTheDocument();
  expect(sqlmapSummary(sqlmap)).toBe("Injetável · 2 técnicas · PostgreSQL");
});

test("sqlmap view shows a clean verdict", () => {
  const clean = { url: "", parametro: "ordem", injetavel: false, sgbd: "", tecnicas: [] };
  render(<SqlmapView report={clean} findings={[]} scan={scan} />);
  expect(screen.getByLabelText("Veredito")).toHaveTextContent("Nenhuma injeção encontrada");
  expect(screen.queryByRole("list", { name: "Técnicas" })).toBeNull();
  expect(sqlmapSummary(clean)).toBe("Nenhuma injeção encontrada");
});

test("sqlmap options explain that real traffic is needed when no route was seen", async () => {
  mockFetch(() => json(200, [route("POST", "/pedidos")]));
  render(<Harness onValue={() => {}} />);
  expect(await screen.findByText(/precisa de tráfego real/)).toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "Rota" })).toBeNull();
  expect(toolViews.sqlmap!.blocked!({})).not.toBeNull();
});

test("sqlmap options pick a GET route, a valid parameter and an app port", async () => {
  mockFetch(() => json(200, [route("GET", "/produtos/:id"), route("POST", "/pedidos"), route("GET", "/busca")]));
  let last: Record<string, string> = {};
  render(<Harness onValue={(v) => (last = v)} />);
  await userEvent.click(await screen.findByRole("combobox", { name: "Rota" }));
  expect(screen.queryByRole("option", { name: "/pedidos" })).toBeNull();
  await userEvent.click(screen.getByRole("option", { name: "/busca" }));
  await userEvent.type(screen.getByLabelText("Parâmetro"), "q --os-shell");
  expect(last.parametro, "nome inválido não segue para a varredura").toBeUndefined();
  expect(toolViews.sqlmap!.blocked!(last)).not.toBeNull();
  await userEvent.clear(screen.getByLabelText("Parâmetro"));
  await userEvent.type(screen.getByLabelText("Parâmetro"), "q");
  await userEvent.click(screen.getByRole("combobox", { name: "Porta" }));
  await userEvent.click(screen.getByRole("option", { name: "8443" }));
  expect(last).toEqual({ rota: "/busca", parametro: "q", porta: "8443" });
  expect(toolViews.sqlmap!.blocked!(last)).toBeNull();
});
