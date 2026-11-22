import type { SqlmapReport } from "../api/reports";
import type { Finding, Scan } from "../api/scans";
import { Notice } from "../components/Card";

export function sqlmapSummary(report: unknown): string {
  const rep = report as SqlmapReport;
  if (!rep.injetavel) return "Nenhuma injeção encontrada";
  const n = rep.tecnicas.length;
  return `Injetável · ${n} ${n === 1 ? "técnica" : "técnicas"}${rep.sgbd ? ` · ${rep.sgbd}` : ""}`;
}

export default function SqlmapView({ report }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const rep = report as SqlmapReport;
  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <p className="text-xs text-suave">
          Parâmetro <span className="font-mono text-texto">{rep.parametro || "?"}</span>
          {rep.url && (
            <>
              {" "}
              em <span className="font-mono break-all text-texto">{rep.url}</span>
            </>
          )}
        </p>
        <p
          aria-label="Veredito"
          className={`text-2xl font-semibold tracking-tight ${rep.injetavel ? "text-erro" : "text-destaque-forte"}`}
        >
          {rep.injetavel ? "Injetável" : "Nenhuma injeção encontrada"}
        </p>
        {rep.sgbd && (
          <p className="text-sm text-suave">
            Banco identificado: <span className="font-mono text-texto">{rep.sgbd}</span>
          </p>
        )}
      </div>
      {rep.tecnicas.length > 0 && (
        <ul aria-label="Técnicas" className="space-y-3">
          {rep.tecnicas.map((t, i) => (
            <li key={i} className="space-y-1 rounded-lg px-2 py-2 text-sm hover:bg-elevado/50">
              <p className="font-medium">{t.tipo}</p>
              <p className="text-suave">{t.titulo}</p>
              <p className="font-mono text-xs break-all text-texto">{t.payload}</p>
            </li>
          ))}
        </ul>
      )}
      <Notice tone="info">
        O teste usou nível 1 e risco 1, sem extrair dados do banco. Um resultado limpo não prova que o parâmetro é seguro: níveis maiores testam
        mais cargas.
      </Notice>
    </div>
  );
}
