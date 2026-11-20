import { useAppStore, type EngineName, type EngineState } from "../store/app";

const engineLabels: Record<EngineName, string> = { poller: "Poller", sniffer: "Sniffer", docker: "Docker" };

const stateLabels: Record<EngineState, string> = {
  ativo: "ativo",
  inativo: "inativo",
  "sem-permissao": "sem permissão",
  indisponivel: "indisponível",
};

const stateColors: Record<EngineState, string> = {
  ativo: "bg-emerald-500",
  inativo: "bg-zinc-600",
  "sem-permissao": "bg-amber-500",
  indisponivel: "bg-red-500",
};

export default function Topbar() {
  const engines = useAppStore((s) => s.engines);
  const clearToken = useAppStore((s) => s.clearToken);
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const selectTarget = useAppStore((s) => s.selectTarget);

  return (
    <header className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-borda bg-superficie px-4 py-3">
      <span className="text-lg font-semibold">Omni</span>
      <select
        aria-label="Alvo"
        value={selected ?? ""}
        onChange={(e) => selectTarget(Number(e.target.value))}
        disabled={targets.length === 0}
        className="max-w-48 rounded border border-borda bg-fundo px-2 py-1 text-sm"
      >
        {targets.length === 0 ? (
          <option value="" disabled>
            Nenhum alvo
          </option>
        ) : (
          targets.map((t) => (
            <option key={t.id} value={t.id}>
              {t.nome}
            </option>
          ))
        )}
      </select>
      <ul className="flex flex-wrap gap-3 text-xs text-suave">
        {(Object.keys(engineLabels) as EngineName[]).map((name) => (
          <li key={name} className="flex items-center gap-1.5">
            <span aria-hidden="true" className={`size-2 rounded-full ${stateColors[engines[name]]}`} />
            {engineLabels[name]}: {stateLabels[engines[name]]}
          </li>
        ))}
      </ul>
      <button
        type="button"
        onClick={() => clearToken(false)}
        className="ml-auto rounded px-2 py-1 text-sm text-suave hover:text-texto"
      >
        Sair
      </button>
    </header>
  );
}
