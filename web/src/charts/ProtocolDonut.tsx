import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import { formatBytes } from "../lib/traffic";
import { chartInk, protocolColor, tooltipStyle } from "./colors";

type Slice = { name: string; bytes: number };

const percent = (part: number, total: number) =>
  `${((100 * part) / total).toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`;

// ProtocolDonut mostra a fatia de bytes de cada protocolo, com legenda em
// texto para que a identidade não dependa só da cor.
export default function ProtocolDonut({ data }: { data: Slice[] }) {
  const total = data.reduce((sum, d) => sum + d.bytes, 0);
  if (total === 0) {
    return <p className="flex h-48 items-center justify-center text-sm text-suave">Sem tráfego capturado ainda.</p>;
  }
  return (
    <div className="flex flex-col items-center gap-4 sm:flex-row">
      <div aria-hidden="true" className="size-44 shrink-0">
        <ResponsiveContainer width="100%" height="100%" initialDimension={{ width: 176, height: 176 }}>
          <PieChart>
            <Pie
              data={data}
              dataKey="bytes"
              nameKey="name"
              innerRadius="74%"
              outerRadius="100%"
              stroke={chartInk.ground}
              strokeWidth={3}
              isAnimationActive={false}
            >
              {data.map((d) => (
                <Cell key={d.name} fill={protocolColor(d.name)} />
              ))}
            </Pie>
            <Tooltip
              {...tooltipStyle}
              formatter={(v, name) => [`${formatBytes(Number(v))} (${percent(Number(v), total)})`, String(name)]}
            />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <ul aria-label="Protocolos" className="w-full space-y-1.5 text-sm">
        {data.map((d) => (
          <li key={d.name} className="flex items-center gap-2">
            <span aria-hidden="true" className="size-2.5 shrink-0 rounded-full" style={{ background: protocolColor(d.name) }} />
            <span className="flex-1">{d.name}</span>
            <span className="whitespace-nowrap font-mono text-xs text-suave tabular-nums">{formatBytes(d.bytes)}</span>
            <span className="w-14 text-right font-mono text-xs tabular-nums">{percent(d.bytes, total)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
