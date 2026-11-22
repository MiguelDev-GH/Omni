import type { NiktoCategory, NiktoItem, NiktoReport } from "../api/reports";
import type { Finding, Scan, Severity } from "../api/scans";
import { SeverityBadge } from "../components/FindingsTable";

export function niktoSummary(report: unknown): string {
  const rep = report as NiktoReport;
  const n = rep.itens.length;
  const items = n === 0 ? "nenhum item" : `${n} ${n === 1 ? "item" : "itens"}`;
  return rep.servidor ? `${rep.servidor} · ${items}` : items.charAt(0).toUpperCase() + items.slice(1);
}

// categories são as categorias do backend (internal/arsenal/nikto.go), na
// ordem exibida, com a severidade que os achados de cada uma recebem.
const categories: { id: NiktoCategory; label: string; severity: Severity }[] = [
  { id: "arquivos", label: "Arquivos expostos", severity: "media" },
  { id: "versao", label: "Versão desatualizada", severity: "media" },
  { id: "cabecalhos", label: "Cabeçalhos", severity: "baixa" },
  { id: "outros", label: "Outros", severity: "info" },
];

function Item({ item }: { item: NiktoItem }) {
  return (
    <li className="space-y-1 rounded-lg px-2 py-2 text-sm hover:bg-elevado/50">
      <p className="flex flex-wrap items-baseline gap-x-2 font-mono text-xs">
        <span className="text-suave">{item.metodo}</span>
        <span className="break-all text-texto">{item.url}</span>
        <span className="text-apagado">#{item.id}</span>
      </p>
      <p>{item.mensagem}</p>
      {item.referencias.length > 0 && (
        <ul aria-label="Referências" className="space-y-0.5">
          {item.referencias.map((ref) => (
            <li key={ref}>
              {/* As referências vêm do banco do Nikto: só http(s) vira link. */}
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
    </li>
  );
}

export default function NiktoView({ report }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const rep = report as NiktoReport;
  return (
    <div className="space-y-8">
      <div>
        <p className="text-xs text-suave">Servidor identificado</p>
        <p aria-label="Servidor identificado" className="font-mono text-2xl font-semibold tracking-tight">
          {rep.servidor || <span className="text-apagado">não informado</span>}
        </p>
      </div>
      {rep.itens.length === 0 ? (
        <p className="text-sm text-suave">O Nikto não relatou nenhum item.</p>
      ) : (
        categories.map((c) => {
          const items = rep.itens.filter((i) => i.categoria === c.id);
          if (items.length === 0) return null;
          return (
            <section key={c.id} aria-label={c.label} className="space-y-2">
              <h3 className="flex items-center gap-2 text-sm font-medium">
                <SeverityBadge s={c.severity} />
                {c.label}
                <span className="font-mono text-xs font-normal text-suave">{items.length}</span>
              </h3>
              <ul className="space-y-px">
                {items.map((it, i) => (
                  <Item key={i} item={it} />
                ))}
              </ul>
            </section>
          );
        })
      )}
    </div>
  );
}
