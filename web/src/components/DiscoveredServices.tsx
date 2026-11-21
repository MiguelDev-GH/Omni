import { useEffect, useState } from "react";
import { listSuggestions, type Suggestion } from "../api/discovery";
import type { Target } from "../api/targets";

type Props = { targets: Target[]; onPick: (s: Suggestion) => void };

// registered diz se todas as portas da sugestão já estão num alvo com o mesmo host.
function registered(s: Suggestion, targets: Target[]): boolean {
  return targets.some(
    (t) =>
      t.host === s.host && s.portas.every((p) => t.portas_app.includes(p) || t.porta_banco === p),
  );
}

export default function DiscoveredServices({ targets, onPick }: Props) {
  const [state, setState] = useState<Suggestion[] | "carregando" | "erro">("carregando");

  // Lista uma vez, ao abrir o formulário.
  useEffect(() => {
    listSuggestions().then(
      (list) => setState(list),
      () => setState("erro"),
    );
  }, []);

  let body;
  if (state === "carregando") {
    body = <p className="text-sm text-suave">Procurando serviços…</p>;
  } else if (state === "erro") {
    body = <p className="text-sm text-suave">Não foi possível listar os serviços desta máquina.</p>;
  } else {
    const visible = state.filter((s) => !registered(s, targets));
    body =
      visible.length === 0 ? (
        <p className="text-sm text-suave">Nenhum serviço novo encontrado.</p>
      ) : (
        <ul className="flex flex-wrap gap-2">
          {visible.map((s) => (
            <li key={`${s.pid ?? 0}-${s.host}-${s.portas.join(",")}`}>
              <button
                type="button"
                onClick={() => onPick(s)}
                className="flex items-center gap-2 rounded-lg bg-elevado px-2.5 py-1.5 text-sm ring-1 ring-borda/60 ring-inset hover:text-destaque-forte hover:ring-destaque/60"
              >
                <span>
                  {s.processo || "Processo desconhecido"} · <span className="font-mono text-xs">{s.portas.join(", ")}</span>
                </span>
                {s.tipo === "banco" && (
                  <span className="rounded-md bg-fundo px-1.5 font-mono text-[10px] text-suave">banco</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      );
  }

  return (
    <section className="space-y-2">
      <h3 className="text-xs font-medium text-suave">Detectado nesta máquina</h3>
      {body}
    </section>
  );
}
