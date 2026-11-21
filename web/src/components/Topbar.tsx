import { useAppStore } from "../store/app";
import EngineBadge, { engineNames } from "./EngineBadge";

export default function Topbar() {
  const engines = useAppStore((s) => s.engines);
  const clearToken = useAppStore((s) => s.clearToken);
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const selectTarget = useAppStore((s) => s.selectTarget);
  const system = useAppStore((s) => s.system);

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
        {engineNames.map((name) => (
          <li key={name}>
            <EngineBadge name={name} state={engines[name]} />
          </li>
        ))}
      </ul>
      {system && (
        <p className="text-xs text-suave tabular-nums">
          CPU {Math.round(system.cpu)}% · Memória {Math.round(system.memoria)}% · Disco {Math.round(system.disco)}%
        </p>
      )}
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
