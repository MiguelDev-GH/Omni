import { createColumnHelper, tableFeatures, useTable } from "@tanstack/react-table";
import { useEffect, useState } from "react";
import { deleteTarget, loadTargets, type Target } from "../api/targets";
import TargetForm from "../components/TargetForm";
import { useAppStore } from "../store/app";

const features = tableFeatures({});
const col = createColumnHelper<typeof features, Target>();
const dash = (v: string | number | undefined) => (v === undefined || v === "" ? "—" : v);
const columns = col.columns([
  col.accessor("nome", { header: "Nome" }),
  col.accessor("host", { header: "Host" }),
  col.accessor((t) => t.portas_app.join(", "), { id: "portas", header: "Portas da aplicação" }),
  col.accessor((t) => dash(t.porta_banco), { id: "banco", header: "Porta do banco" }),
  col.accessor((t) => dash(t.dominio), { id: "dominio", header: "Domínio" }),
]);

type Editing = { mode: "novo" } | { mode: "editar"; target: Target } | null;

export default function TargetsPage() {
  const targets = useAppStore((s) => s.targets);
  const loaded = useAppStore((s) => s.targetsLoaded);
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
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-xl font-semibold">Alvos</h1>
        {editing === null && (
          <button
            type="button"
            onClick={() => setEditing({ mode: "novo" })}
            className="rounded bg-destaque px-3 py-1.5 font-medium text-zinc-950"
          >
            Novo alvo
          </button>
        )}
      </div>

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
        <p className="text-suave">Nenhum alvo cadastrado. Cadastre o primeiro para começar o monitoramento.</p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-borda">
          <table className="w-full text-left text-sm">
            <thead className="bg-superficie text-suave">
              {table.getHeaderGroups().map((hg) => (
                <tr key={hg.id}>
                  {hg.headers.map((h) => (
                    <th key={h.id} scope="col" className="px-3 py-2 font-medium whitespace-nowrap">
                      <table.FlexRender header={h} />
                    </th>
                  ))}
                  <th scope="col" className="px-3 py-2 font-medium">
                    Ações
                  </th>
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map((row) => (
                <tr key={row.id} className="border-t border-borda">
                  {row.getAllCells().map((cell) => (
                    <td key={cell.id} className="max-w-64 truncate px-3 py-2">
                      <table.FlexRender cell={cell} />
                    </td>
                  ))}
                  <td className="px-3 py-2">
                    <div className="flex gap-3">
                      <button
                        type="button"
                        aria-label={`Editar ${row.original.nome}`}
                        onClick={() => setEditing({ mode: "editar", target: row.original })}
                        className="text-suave hover:text-texto"
                      >
                        Editar
                      </button>
                      <button
                        type="button"
                        aria-label={`Excluir ${row.original.nome}`}
                        onClick={() => void remove(row.original)}
                        className="text-erro hover:underline"
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
