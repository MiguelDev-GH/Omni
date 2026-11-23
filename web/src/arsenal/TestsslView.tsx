import type { TestsslReport } from "../api/reports";
import type { Finding, Scan } from "../api/scans";

// insecure são os protocolos que não deveriam ser oferecidos.
const insecure = new Set(["SSLv2", "SSLv3", "TLS 1.0", "TLS 1.1"]);

// Faixa da barra de validade: um ano de certificado enche a barra.
const FULL_DAYS = 365;

export function testsslSummary(report: unknown): string {
  const rep = report as TestsslReport;
  const grade = `Nota ${rep.nota || "—"}`;
  if (!rep.certificado.valido_ate) return grade;
  const d = rep.certificado.dias_restantes;
  return d < 0 ? `${grade} · certificado vencido` : `${grade} · certificado vence em ${d} ${d === 1 ? "dia" : "dias"}`;
}

function gradeTone(grade: string): string {
  if (grade.startsWith("A")) return "text-destaque-forte";
  if (grade === "B" || grade === "C") return "text-aviso";
  if (grade === "") return "text-apagado";
  return "text-erro";
}

function daysTone(days: number): { bar: string; text: string } {
  if (days < 7) return { bar: "bg-erro", text: "text-erro" };
  if (days < 30) return { bar: "bg-aviso", text: "text-aviso" };
  return { bar: "bg-destaque", text: "text-destaque-forte" };
}

export default function TestsslView({ report }: { report: unknown; findings: Finding[]; scan: Scan }) {
  const rep = report as TestsslReport;
  const cert = rep.certificado;
  const vulnerable = rep.vulnerabilidades.filter((v) => v.vulneravel).length;

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end gap-x-10 gap-y-6">
        <div>
          <p className="text-xs text-suave">Nota geral</p>
          <p aria-label="Nota geral" className={`font-mono text-6xl font-semibold tracking-tight ${gradeTone(rep.nota)}`}>
            {rep.nota || "—"}
          </p>
          {rep.nota === "T" && <p className="mt-1 text-xs text-suave">T: o certificado não é confiável.</p>}
        </div>

        <div className="min-w-64 flex-1 space-y-2">
          <p className="text-xs text-suave">Certificado</p>
          {cert.valido_ate ? (
            <>
              {(() => {
                const tone = daysTone(cert.dias_restantes);
                const pct = Math.max(0, Math.min(100, (cert.dias_restantes / FULL_DAYS) * 100));
                return (
                  <>
                    <p className={`text-sm font-medium ${tone.text}`}>
                      {cert.dias_restantes < 0
                        ? "Vencido"
                        : `${cert.dias_restantes} ${cert.dias_restantes === 1 ? "dia restante" : "dias restantes"}`}
                      <span className="font-normal text-suave">
                        {" "}
                        · até {new Date(cert.valido_ate).toLocaleDateString("pt-BR")}
                      </span>
                    </p>
                    <div
                      role="meter"
                      aria-label="Validade do certificado"
                      aria-valuemin={0}
                      aria-valuemax={FULL_DAYS}
                      aria-valuenow={Math.max(0, cert.dias_restantes)}
                      className="h-1.5 rounded-full bg-texto/10"
                    >
                      <div className={`h-full rounded-full ${tone.bar}`} style={{ width: `${Math.max(pct, 1)}%` }} />
                    </div>
                  </>
                );
              })()}
            </>
          ) : (
            <p className="text-sm text-suave">Validade não informada.</p>
          )}
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
            <dt className="text-suave">Assunto</dt>
            <dd className="font-mono text-xs leading-5">{cert.assunto || "—"}</dd>
            <dt className="text-suave">Emissor</dt>
            <dd className="font-mono text-xs leading-5">{cert.emissor || "—"}</dd>
            <dt className="text-suave">Confiança</dt>
            <dd className={cert.confiavel ? "text-destaque-forte" : "text-erro"}>
              {cert.confiavel ? "Cadeia confiável" : "Cadeia não confiável"}
            </dd>
          </dl>
        </div>
      </div>

      <section>
        <h3 className="mb-3 text-xs text-suave">Protocolos</h3>
        <ol aria-label="Protocolos" className="grid grid-cols-3 gap-2 sm:grid-cols-6">
          {rep.protocolos.map((p) => {
            const problem = p.oferecido && insecure.has(p.nome);
            const tone = problem ? "bg-critica-tinta text-critica-texto" : p.oferecido ? "bg-destaque-tinta text-destaque-forte" : "bg-texto/5 text-suave";
            return (
              <li key={p.nome} className={`rounded-xl px-3 py-2.5 ${tone}`}>
                <p className="font-mono text-sm font-medium">{p.nome}</p>
                <p className="text-xs">{problem ? "Oferecido: inseguro" : p.oferecido ? "Oferecido" : "Não oferecido"}</p>
              </li>
            );
          })}
        </ol>
      </section>

      <section>
        <h3 className="mb-3 text-xs text-suave">
          Vulnerabilidades testadas ·{" "}
          {vulnerable === 0 ? "nenhuma encontrada" : `${vulnerable} ${vulnerable === 1 ? "encontrada" : "encontradas"}`}
        </h3>
        <ul aria-label="Vulnerabilidades testadas" className="grid gap-x-6 gap-y-1 text-sm sm:grid-cols-2 lg:grid-cols-3">
          {rep.vulnerabilidades.map((v) => (
            <li key={v.id} className="flex items-baseline justify-between gap-3 py-1.5">
              <span>
                {v.nome}
                {v.cve && <span className="block font-mono text-[11px] text-apagado">{v.cve.split(" ")[0]}</span>}
              </span>
              <span className={`text-xs font-medium whitespace-nowrap ${v.vulneravel ? "text-erro" : "text-suave"}`}>
                {v.vulneravel ? "Vulnerável" : "Não vulnerável"}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
