import { useEffect, useState } from "react";
import { listRoutes } from "../api/requests";
import type { Target } from "../api/targets";
import { Notice } from "../components/Card";
import Select from "../components/Select";
import { PortField } from "./common";

// parameter é o padrão que o backend aceita para o nome do parâmetro.
const parameter = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/;

export default function SqlmapOptions({
  value,
  onChange,
  target,
}: {
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  target: Target;
}) {
  // routes fica null enquanto carrega e vazio quando o sniffer não viu
  // nenhuma rota GET (ou a busca falhou): sem rota vista, não há o que testar.
  const [routes, setRoutes] = useState<string[] | null>(null);
  const [text, setText] = useState(value.parametro ?? "");

  useEffect(() => {
    let live = true;
    listRoutes().then(
      (list) => live && setRoutes([...new Set(list.filter((r) => r.metodo === "GET").map((r) => r.rota))].sort()),
      () => live && setRoutes([]),
    );
    return () => {
      live = false;
    };
  }, []);

  if (routes === null) return <p className="text-sm text-suave">Carregando as rotas vistas no tráfego...</p>;
  if (routes.length === 0)
    return (
      <Notice tone="info">
        O SQLMap precisa de tráfego real na aplicação: nenhuma rota GET foi vista nas últimas 24 horas. Use a aplicação com o sniffer ativo e volte aqui.
      </Notice>
    );

  const invalid = text !== "" && !parameter.test(text);
  // Só um parâmetro válido segue para a varredura: um nome inválido some
  // das opções e deixa o botão desabilitado.
  const setParam = (v: string) => {
    setText(v);
    const next: Record<string, string | undefined> = { ...value, parametro: v };
    if (!parameter.test(v)) delete next.parametro;
    onChange(next as Record<string, string>);
  };

  return (
    <div className="flex flex-wrap items-end gap-x-6 gap-y-3 text-sm">
      <div className="flex items-center gap-3">
        <label htmlFor="sqlmap-rota" className="text-suave">
          Rota
        </label>
        <Select
          id="sqlmap-rota"
          value={value.rota ?? null}
          onChange={(rota) => onChange({ ...value, rota })}
          options={routes.map((r) => ({ value: r, label: r }))}
          placeholder="Escolha uma rota"
        />
      </div>
      <div className="flex items-center gap-3">
        <label htmlFor="sqlmap-parametro" className="text-suave">
          Parâmetro
        </label>
        <input
          id="sqlmap-parametro"
          value={text}
          onChange={(e) => setParam(e.target.value)}
          placeholder="ex.: q"
          maxLength={64}
          autoComplete="off"
          spellCheck={false}
          aria-invalid={invalid}
          className="campo font-mono"
        />
      </div>
      <PortField id="sqlmap-porta" value={value} onChange={onChange} target={target} />
    </div>
  );
}
