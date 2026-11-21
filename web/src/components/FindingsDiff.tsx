import { sortFindings, type Comparison, type Finding } from "../api/scans";
import { SeverityBadge } from "./FindingsTable";

function Row({ f, resolved }: { f: Finding; resolved?: boolean }) {
  return (
    <li className={`flex items-start gap-2 py-1.5 ${resolved ? "text-suave line-through" : ""}`}>
      <SeverityBadge s={f.severidade} />
      <span className="min-w-0 flex-1">
        {f.titulo}
        <span className="ml-2 text-xs text-suave no-underline">
          {f.ferramenta}
          {f.porta ? ` · porta ${f.porta}` : ""}
        </span>
      </span>
    </li>
  );
}

// FindingsDiff mostra o que mudou desde a varredura anterior: achados novos
// em destaque e resolvidos riscados, os mais graves primeiro.
export default function FindingsDiff({ cmp }: { cmp: Comparison }) {
  const first = Object.keys(cmp.anteriores).length === 0;
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <section aria-label="Achados novos" className="rounded-lg border border-amber-500/40 bg-amber-500/5 p-3">
        <h3 className="mb-1 text-sm font-medium text-amber-200">
          Novos ({cmp.novos.length}){first && " · primeira varredura deste alvo"}
        </h3>
        {cmp.novos.length === 0 ? (
          <p className="text-sm text-suave">Nada novo desde a varredura anterior.</p>
        ) : (
          <ul className="divide-y divide-borda text-sm">
            {sortFindings(cmp.novos).map((f) => (
              <Row key={f.assinatura} f={f} />
            ))}
          </ul>
        )}
      </section>
      <section aria-label="Achados resolvidos" className="rounded-lg border border-borda p-3">
        <h3 className="mb-1 text-sm font-medium text-destaque">Resolvidos ({cmp.resolvidos.length})</h3>
        {cmp.resolvidos.length === 0 ? (
          <p className="text-sm text-suave">Nenhum achado da varredura anterior sumiu.</p>
        ) : (
          <ul className="divide-y divide-borda text-sm">
            {sortFindings(cmp.resolvidos).map((f) => (
              <Row key={f.assinatura} f={f} resolved />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
