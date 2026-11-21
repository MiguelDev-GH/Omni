import { createColumnHelper, tableFeatures, useTable } from "@tanstack/react-table";
import { useEffect, useState } from "react";
import { deleteTarget, loadTargets, targetPorts, type Target } from "../api/targets";
import { PageHeader } from "../components/Card";
import TargetForm from "../components/TargetForm";
import { portKey, useAppStore, type PortStatus } from "../store/app";

const features = tableFeatures({});
const col = createColumnHelper<typeof features, Target>();
const dash = (v: string | number | undefined) => (v === undefined || v === "" ? "—" : v);
const columns = col.columns([
  col.accessor("nome", { header: "Nome" }),
  col.accessor("host", { header: "Host" }),
  col.accessor((t) => t.portas_app.join(", "), { id: "portas", header: "Portas da aplicação" }),
  col.accessor((t) => dash(t.porta_banco), { id: "banco", header: "Porta do banco" }),
  col.accessor((t) => dash(t.dominio), { id: "dominio", header: "Domínio" }),
  col.accessor((t) => dnsSmtp(t), { id: "dns_smtp", header: "DNS / SMTP" }),
]);

function dnsSmtp(t: Target): string {
  const parts = [t.porta_dns ? `DNS ${t.porta_dns}` : "", t.porta_smtp ? `SMTP ${t.porta_smtp}` : ""].filter(Boolean);
  return parts.length === 0 ? "—" : parts.join(" · ");
}

// targetState resume o último status das portas do alvo.
function targetState(t: Target, status: Record<string, PortStatus>): { text: string; color: string; dot: string } {
  const known = targetPorts(t)
    .map((p) => status[portKey(t.id, p)])
    .filter((s) => s !== undefined);
  if (known.length === 0) return { text: "Aguardando", color: "text-suave", dot: "bg-apagado" };
  const down = known.filter((s) => !s.no_ar).map((s) => s.porta);
  if (down.length > 0) return { text: `Fora do ar (porta ${down.join(", ")})`, color: "text-erro", dot: "bg-erro animate-pulso" };
  const degraded = known.filter((s) => s.degradado).map((s) => s.porta);
  if (degraded.length > 0) return { text: `Com erro (porta ${degraded.join(", ")})`, color: "text-aviso", dot: "bg-aviso" };
  return { text: "No ar", color: "text-destaque-forte", dot: "bg-destaque" };
}

type Editing = { mode: "novo" } | { mode: "editar"; target: Target } | null;

export default function TargetsPage() {
  const targets = useAppStore((s) => s.targets);
  const loaded = useAppStore((s) => s.targetsLoaded);
  const status = useAppStore((s) => s.targetStatus);
  const [editing, setEditing] = useState<Editing>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  function reload(): Promise<void> {
    return loadTargets().then(
      () => setLoadError(null),
      () => setLoadError("Não foi possível carregar os alvos."),
    );
  }

  // Carrega a lista uma vez, ao montar a página.
  useEffect(() => {
    reload();
  }, []);

  async function remove(t: Target) {
    if (!window.confirm(`Excluir o alvo "${t.nome}"? O histórico de verificações dele também será apagado.`)) return;
    try {
      await deleteTarget(t.id);
    } catch {
      setLoadError(`Não foi possível excluir o alvo "${t.nome}".`);
    }
    await reload();
  }

  const table = useTable({ features, columns, data: targets });

  function done() {
    setEditing(null);
    void reload();
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Alvos"
        aside={
          editing === null && (
            <button type="button" onClick={() => setEditing({ mode: "novo" })} className="botao">
              Novo alvo
            </button>
          )
        }
      />

      {editing && (
        <TargetForm
          key={editing.mode === "editar" ? editing.target.id : "novo"}
          initial={editing.mode === "editar" ? editing.target : undefined}
          onDone={done}
          onCancel={() => setEditing(null)}
        />
      )}

      {loadError && <p className="text-sm text-erro">{loadError}</p>}

      {loaded && targets.length === 0 ? (
        <p className="rounded-2xl bg-superficie/80 px-5 py-8 text-center text-suave">Nenhum alvo cadastrado. Cadastre o primeiro para começar o monitoramento.</p>
      ) : (
        <div className="overflow-x-auto rounded-2xl bg-superficie/80 px-5 pt-4 pb-2">
          <table className="tabela">
            <thead>
              {table.getHeaderGroups().map((hg) => (
                <tr key={hg.id}>
                  {hg.headers.map((h) => (
                    <th key={h.id} scope="col" className="pr-4 whitespace-nowrap">
                      <table.FlexRender header={h} />
                    </th>
                  ))}
                  <th scope="col" className="pr-4">
                    Estado
                  </th>
                  <th scope="col" className="text-right">
                    Ações
                  </th>
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map((row) => (
                <tr key={row.id}>
                  {row.getAllCells().map((cell) => (
                    <td key={cell.id} className="max-w-64 truncate py-2.5 pr-4">
                      <table.FlexRender cell={cell} />
                    </td>
                  ))}
                  <td className={`pr-4 whitespace-nowrap ${targetState(row.original, status).color}`}>
                    <span className="inline-flex items-center gap-2">
                      <span aria-hidden="true" className={`size-1.5 rounded-full ${targetState(row.original, status).dot}`} />
                      {targetState(row.original, status).text}
                    </span>
                  </td>
                  <td>
                    <div className="flex justify-end gap-1">
                      <button
                        type="button"
                        aria-label={`Editar ${row.original.nome}`}
                        onClick={() => setEditing({ mode: "editar", target: row.original })}
                        className="botao-leve px-2 py-1"
                      >
                        Editar
                      </button>
                      <button
                        type="button"
                        aria-label={`Excluir ${row.original.nome}`}
                        onClick={() => void remove(row.original)}
                        className="botao-leve px-2 py-1 text-erro hover:bg-erro/10 hover:text-erro"
                      >
                        Excluir
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
