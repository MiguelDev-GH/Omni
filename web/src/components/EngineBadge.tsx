import type { EngineName, EngineState } from "../store/app";

const engineLabels: Record<EngineName, string> = { poller: "Poller", sniffer: "Sniffer", docker: "Docker" };

const stateLabels: Record<EngineState, string> = {
  ativo: "ativo",
  inativo: "inativo",
  "sem-permissao": "sem permissão",
  indisponivel: "indisponível",
};

const stateColors: Record<EngineState, string> = {
  ativo: "bg-destaque text-destaque animate-pulso",
  inativo: "bg-apagado",
  "sem-permissao": "bg-aviso",
  indisponivel: "bg-erro",
};

export const engineNames = Object.keys(engineLabels) as EngineName[];

// EngineBadge mostra o estado de um motor com cor e texto, nunca só com a cor.
export default function EngineBadge({ name, state }: { name: EngineName; state: EngineState }) {
  return (
    <span className="flex items-center gap-1.5">
      <span aria-hidden="true" className={`size-1.5 rounded-full ${stateColors[state]}`} />
      {engineLabels[name]}: {stateLabels[state]}
    </span>
  );
}
