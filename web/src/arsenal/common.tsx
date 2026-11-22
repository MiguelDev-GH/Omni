import type { Scan } from "../api/scans";
import type { Target } from "../api/targets";
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

// PortField escolhe a opção "porta" entre as portas de aplicação do alvo;
// sem escolha, mostra a primeira, que é o padrão do backend.
export function PortField({
  id,
  value,
  onChange,
  target,
}: {
  id: string;
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  target: Target;
}) {
  return (
    <div className="flex items-center gap-3 text-sm">
      <label htmlFor={id} className="text-suave">
        Porta
      </label>
      <Select
        id={id}
        value={value.porta ?? String(target.portas_app[0] ?? "")}
        onChange={(porta) => onChange({ ...value, porta })}
        options={target.portas_app.map((p) => ({ value: String(p), label: String(p) }))}
      />
    </div>
  );
}
