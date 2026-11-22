import type { Scan } from "../api/scans";
import Select from "../components/Select";
import { useAppStore } from "../store/app";

// statusLabels traduz o estado de uma varredura para a interface.
export const statusLabels: Record<Scan["status"], string> = {
  fila: "Na fila",
  rodando: "Rodando",
  concluida: "Concluída",
  falhou: "Falhou",
};

export const when = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "medium" });

export const isRunning = (s: Scan | undefined) => s?.status === "rodando" || s?.status === "fila";

// TargetPicker escolhe o alvo das varreduras, o mesmo selecionado no resto da interface.
export function TargetPicker({ id }: { id: string }) {
  const targets = useAppStore((s) => s.targets);
  const selected = useAppStore((s) => s.selectedTargetId);
  const selectTarget = useAppStore((s) => s.selectTarget);
  return (
    <div className="flex flex-wrap items-center gap-3 text-sm">
      <label htmlFor={id} className="text-suave">
        Alvo
      </label>
      <Select
        id={id}
        value={selected}
        onChange={selectTarget}
        options={targets.map((t) => ({ value: t.id, label: t.nome, hint: t.host }))}
      />
    </div>
  );
}
