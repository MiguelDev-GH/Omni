import { useAppStore } from "../store/app";
import EngineBadge, { engineNames } from "./EngineBadge";
import Logo from "./Logo";
import Select from "./Select";
import ThemeSwitch from "./ThemeSwitch";

// Meter é um uso de recurso em barra fina, com o número ao lado.
function Meter({ label, value }: { label: string; value: number }) {
  const v = Math.round(value);
  return (
    <span className="flex items-center gap-1.5" title={`${label} ${v}%`}>
      <span>{label}</span>
      <span aria-hidden="true" className="h-1 w-8 overflow-hidden rounded-full bg-borda">
        <span
          className={`block h-full rounded-full transition-[width] duration-700 ease-(--ease-suave) ${v >= 90 ? "bg-erro" : v >= 75 ? "bg-aviso" : "bg-destaque"}`}
          style={{ width: `${Math.min(v, 100)}%` }}
        />
      </span>
      <span className="w-8 text-texto tabular-nums">{v}%</span>
    </span>
  );
}

export default function Topbar({ menuOpen, onToggleMenu }: { menuOpen: boolean; onToggleMenu: () => void }) {
  const engines = useAppStore((s) => s.engines);
  const clearToken = useAppStore((s) => s.clearToken);
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const selectTarget = useAppStore((s) => s.selectTarget);
  const system = useAppStore((s) => s.system);

  return (
    <header className="sticky top-0 z-30 bg-fundo/85 backdrop-blur-md">
      <div className="mx-auto flex min-h-16 max-w-[1500px] flex-wrap items-center gap-x-6 gap-y-2 px-4 py-2.5">
        <button
          type="button"
          onClick={onToggleMenu}
          aria-expanded={menuOpen}
          aria-controls="menu"
          aria-label="Menu"
          className="botao-leve -ml-2 px-2 md:hidden"
        >
          <svg aria-hidden="true" viewBox="0 0 16 16" className="size-4">
            <path d="M2 4.5h12M2 8h12M2 11.5h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
        </button>
        <span className="flex items-center md:w-48">
          <Logo className="h-8 w-auto" />
        </span>
        <Select
          label="Alvo"
          value={selected}
          onChange={selectTarget}
          options={targets.map((t) => ({ value: t.id, label: t.nome, hint: t.host }))}
          placeholder="Nenhum alvo"
          className="max-w-56"
        />
        <ul className="order-last flex w-full flex-wrap gap-x-4 gap-y-1 text-xs text-suave md:order-none md:w-auto">
          {engineNames.map((name) => (
            <li key={name}>
              <EngineBadge name={name} state={engines[name]} />
            </li>
          ))}
        </ul>
        <div className="ml-auto flex items-center gap-5">
          {system && (
            <p className="hidden items-center gap-4 font-mono text-[11px] text-suave lg:flex">
              <Meter label="CPU" value={system.cpu} />
              <Meter label="Mem" value={system.memoria} />
              <Meter label="Disco" value={system.disco} />
            </p>
          )}
          <div className="flex items-center gap-1">
            <ThemeSwitch />
            <button type="button" onClick={() => clearToken(false)} className="botao-leve">
              Sair
            </button>
          </div>
        </div>
      </div>
    </header>
  );
}
