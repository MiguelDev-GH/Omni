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
                className="flex items-center gap-2 rounded border border-borda bg-fundo px-2.5 py-1 text-sm hover:border-destaque"
              >
                <span>
                  {s.processo || "Processo desconhecido"} · {s.portas.join(", ")}
                </span>
                {s.tipo === "banco" && (
                  <span className="rounded bg-zinc-800 px-1.5 text-[10px] tracking-wide text-suave uppercase">banco</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      );
  }

  return (
    <section className="space-y-2">
      <h3 className="text-sm font-medium text-suave">Detectado nesta máquina</h3>
      {body}
    </section>
  );
}
