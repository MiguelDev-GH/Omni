import type { Target } from "../api/targets";
import Select from "../components/Select";
import { PortField } from "./common";

// lists são as listas de palavras embutidas no backend
// (internal/arsenal/wordlists), com o número de palavras de cada uma.
const lists = [
  { value: "pequena", label: "Pequena", hint: "4.751 palavras · cerca de 1 min" },
  { value: "media", label: "Média", hint: "43.007 palavras · cerca de 7 min" },
];

export default function FfufOptions({
  value,
  onChange,
  target,
}: {
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  target: Target;
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-6 gap-y-3 text-sm">
      <div className="flex items-center gap-3">
        <label htmlFor="ffuf-lista" className="text-suave">
          Lista de palavras
        </label>
        <Select id="ffuf-lista" value={value.lista ?? "pequena"} onChange={(lista) => onChange({ ...value, lista })} options={lists} />
      </div>
      <PortField id="ffuf-porta" value={value} onChange={onChange} target={target} />
    </div>
  );
}
