import { useState } from "react";
import type { GitleaksReport, Leak } from "../api/reports";
import type { Finding, Scan } from "../api/scans";
import Select from "../components/Select";
import { when } from "./common";

export function gitleaksSummary(report: unknown): string {
  const list = (report as GitleaksReport).vazamentos;
  if (list.length === 0) return "Nenhum segredo encontrado";
  const files = new Set(list.map((l) => l.arquivo)).size;
  return `${list.length} ${list.length === 1 ? "vazamento" : "vazamentos"} em ${files} ${files === 1 ? "arquivo" : "arquivos"}`;
}

// byFile agrupa os vazamentos por arquivo, em ordem de caminho e de linha.
export function byFile(list: Leak[]): [string, Leak[]][] {
  const groups = new Map<string, Leak[]>();
  for (const l of list) groups.set(l.arquivo, [...(groups.get(l.arquivo) ?? []), l]);
  return [...groups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([file, leaks]) => [file, leaks.sort((a, b) => a.linha - b.linha)]);
}

const ALL = "";

// LeakRow mostra só os campos conhecidos do vazamento: mesmo que um relatório
// traga indevidamente o valor, ele nunca é lido aqui.
function LeakRow({ leak }: { leak: Leak }) {
  return (
    <li className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 rounded-lg px-2 py-1.5 text-sm hover:bg-elevado/50">
      <span className="w-14 shrink-0 font-mono text-xs text-suave">linha {leak.linha}</span>
      <span className="font-mono text-xs" title={leak.descricao}>
        {leak.regra}
      </span>
      <span className="rounded-md bg-texto/5 px-1.5 py-0.5 text-[11px] text-apagado">valor ocultado</span>
      {leak.commit ? (
        <span className="text-xs text-suave">
          <span className="font-mono" title={leak.commit}>
            {leak.commit.slice(0, 7)}
          </span>
          {leak.autor && ` · ${leak.autor}`}
          {leak.data && ` · ${when(leak.data)}`}
        </span>
      ) : (
        <span className="text-xs text-apagado">fora do histórico do git</span>
      )}
    </li>
  );
}

export default function GitleaksView({ report }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const list = (report as GitleaksReport).vazamentos;
  const [rule, setRule] = useState<string>(ALL);

  if (list.length === 0) return <p className="text-sm text-suave">O Gitleaks não encontrou nenhum segredo no código-fonte.</p>;

  const rules = [...new Set(list.map((l) => l.regra))].sort();
  const shown = list.filter((l) => rule === ALL || l.regra === rule);

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center gap-x-5 gap-y-3 text-sm">
        <p className="text-suave">{gitleaksSummary(report)}</p>
        {rules.length > 1 && (
          <Select
            label="Regra"
            value={rule}
            onChange={setRule}
            options={[{ value: ALL, label: "Todas as regras" }, ...rules.map((r) => ({ value: r, label: r }))]}
          />
        )}
      </div>
      <ul aria-label="Arquivos" className="space-y-4">
        {byFile(shown).map(([file, leaks]) => (
          <li key={file} aria-label={file}>
            <p className="px-2 font-mono text-sm break-all text-texto">
              {file}
              <span className="ml-2 text-xs text-suave">
                {leaks.length} {leaks.length === 1 ? "vazamento" : "vazamentos"}
              </span>
            </p>
            <ul className="mt-1 ml-2 space-y-px border-l border-borda pl-2">
              {leaks.map((l) => (
                <LeakRow key={`${l.linha}-${l.regra}-${l.commit}`} leak={l} />
              ))}
            </ul>
          </li>
        ))}
      </ul>
      <p className="text-xs text-apagado">
        O valor de cada segredo nunca é guardado nem mostrado. Troque a credencial e apague-a do histórico do git.
      </p>
    </div>
  );
}
