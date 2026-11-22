import { useState } from "react";
import type { NucleiReport, NucleiResult } from "../api/reports";
import { severities, severityLabels, type Finding, type Scan, type Severity } from "../api/scans";
import { SeverityBadge } from "../components/FindingsTable";
import Select from "../components/Select";

export function nucleiSummary(report: unknown): string {
  const list = (report as NucleiReport).resultados;
  const templates = new Set(list.map((r) => r.template)).size;
  if (list.length === 0) return "Nenhum template casou";
  return `${list.length} ${list.length === 1 ? "resultado" : "resultados"} em ${templates} ${templates === 1 ? "template" : "templates"}`;
}

type Group = { template: string; nome: string; severidade: Severity; descricao: string; tags: string[]; results: NucleiResult[] };

// group junta os resultados por template, dos mais graves aos menos graves.
function group(list: NucleiResult[]): Group[] {
  const by = new Map<string, Group>();
  for (const r of list) {
    const g = by.get(r.template);
    if (g) g.results.push(r);
    else by.set(r.template, { template: r.template, nome: r.nome || r.template, severidade: r.severidade, descricao: r.descricao, tags: r.tags, results: [r] });
  }
  const rank = (s: Severity) => severities.indexOf(s);
  return [...by.values()].sort((a, b) => rank(a.severidade) - rank(b.severidade) || a.nome.localeCompare(b.nome));
}

const ALL = "";

export default function NucleiView({ report }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const list = (report as NucleiReport).resultados;
  const [hidden, setHidden] = useState<Severity[]>([]);
  const [tag, setTag] = useState<string>(ALL);

  if (list.length === 0) return <p className="text-sm text-suave">Nenhum template do Nuclei casou com o alvo.</p>;

  const present = severities.filter((s) => list.some((r) => r.severidade === s));
  const tags = [...new Set(list.flatMap((r) => r.tags))].sort();
  const shown = list.filter((r) => !hidden.includes(r.severidade) && (tag === ALL || r.tags.includes(tag)));
  const toggle = (s: Severity) => setHidden((h) => (h.includes(s) ? h.filter((x) => x !== s) : [...h, s]));

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center gap-x-5 gap-y-3 text-sm">
        <fieldset className="flex flex-wrap items-center gap-1.5">
          <legend className="sr-only">Severidade</legend>
          {present.map((s) => (
            <label
              key={s}
              className="flex cursor-pointer items-center gap-1.5 rounded-lg px-2 py-1 hover:bg-elevado/70 has-checked:bg-elevado"
            >
              <input type="checkbox" checked={!hidden.includes(s)} onChange={() => toggle(s)} className="size-3.5" />
              {severityLabels[s]}
              <span className="font-mono text-xs text-suave">{list.filter((r) => r.severidade === s).length}</span>
            </label>
          ))}
        </fieldset>
        {tags.length > 0 && (
          <Select
            label="Tag"
            value={tag}
            onChange={setTag}
            options={[{ value: ALL, label: "Todas as tags" }, ...tags.map((t) => ({ value: t, label: t }))]}
          />
        )}
      </div>

      {shown.length === 0 ? (
        <p className="text-sm text-suave">Nenhum resultado com estes filtros.</p>
      ) : (
        <ul aria-label="Templates" className="space-y-1">
          {group(shown).map((g) => (
            <li key={g.template}>
              <details className="group rounded-xl px-3 py-2.5 open:bg-superficie/80 hover:bg-elevado/50">
                <summary className="flex cursor-pointer list-none flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                  <SeverityBadge s={g.severidade} />
                  <span className="font-medium">{g.nome}</span>
                  <span className="font-mono text-xs text-suave">{g.template}</span>
                  <span className="ml-auto font-mono text-xs text-suave">
                    {g.results.length} {g.results.length === 1 ? "ocorrência" : "ocorrências"}
                  </span>
                </summary>
                <div className="mt-3 space-y-3 text-sm">
                  {g.descricao && <p className="text-suave">{g.descricao}</p>}
                  {g.tags.length > 0 && (
                    <p className="flex flex-wrap gap-1">
                      {g.tags.map((t) => (
                        <span key={t} className="rounded-md bg-texto/5 px-1.5 py-0.5 font-mono text-[11px] text-suave">
                          {t}
                        </span>
                      ))}
                    </p>
                  )}
                  <ul className="space-y-1.5">
                    {g.results.map((r, i) => (
                      <li key={i} className="font-mono text-xs break-all">
                        {r.url}
                        {r.matcher && <span className="text-suave"> · {r.matcher}</span>}
                        {r.extraidos.length > 0 && <span className="block text-suave">extraído: {r.extraidos.join(", ")}</span>}
                      </li>
                    ))}
                  </ul>
                  {g.results[0]!.referencias.length > 0 && (
                    <ul aria-label="Referências" className="space-y-0.5">
                      {g.results[0]!.referencias.map((ref) => (
                        <li key={ref}>
                          {/* As referências vêm dos templates: só http(s) vira link. */}
                          {/^https?:\/\//i.test(ref) ? (
                            <a href={ref} target="_blank" rel="noreferrer" className="link text-xs break-all">
                              {ref}
                            </a>
                          ) : (
                            <span className="font-mono text-xs break-all text-suave">{ref}</span>
                          )}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </details>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
