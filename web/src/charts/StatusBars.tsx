import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { statusClasses, type StatusPoint } from "../lib/l7";
import { axisTick, chartInk, statusColors, tooltipStyle } from "./colors";

const clock = (t: number) => new Date(t).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
const clockSec = (t: number) => new Date(t).toLocaleTimeString("pt-BR");

// StatusBars empilha as respostas de cada intervalo de 10 s por classe de
// status, com legenda em texto e os totais do período.
export default function StatusBars({ points }: { points: (StatusPoint & { t: number })[] }) {
  const totals = statusClasses.map((c) => ({ c, n: points.reduce((sum, p) => sum + p[c], 0) }));
  if (totals.every((x) => x.n === 0)) {
    return <p className="flex h-52 items-center justify-center text-sm text-suave">Sem requisições capturadas ainda.</p>;
  }
  return (
    <div className="space-y-3">
      <div role="img" aria-label={`Status HTTP a cada 10 s, ${points.length} pontos`} className="h-52">
        <ResponsiveContainer width="100%" height="100%" initialDimension={{ width: 600, height: 208 }}>
          <BarChart data={points} margin={{ top: 8, right: 8, bottom: 0, left: 0 }} barCategoryGap={2}>
            <CartesianGrid stroke={chartInk.grid} strokeDasharray="1 5" vertical={false} />
            <XAxis
              dataKey="t"
              tickFormatter={clock}
              axisLine={false}
              tick={axisTick}
              tickLine={false}
              minTickGap={48}
            />
            <YAxis allowDecimals={false} tick={axisTick} tickLine={false} axisLine={false} width={48} />
            <Tooltip
              cursor={{ fill: "rgb(22 33 30 / 0.04)" }}
              {...tooltipStyle}
              labelFormatter={(t) => clockSec(Number(t))}
            />
            {statusClasses.map((c) => (
              <Bar key={c} dataKey={c} stackId="status" fill={statusColors[c]} fillOpacity={0.8} isAnimationActive={false} />
            ))}
          </BarChart>
        </ResponsiveContainer>
      </div>
      <ul aria-label="Status HTTP" className="flex flex-wrap gap-x-5 gap-y-1 text-sm">
        {totals.map(({ c, n }) => (
          <li key={c} className="flex items-center gap-2">
            <span aria-hidden="true" className="size-2.5 rounded-full" style={{ background: statusColors[c] }} />
            <span>{c}</span>
            <span className="font-mono text-xs text-suave tabular-nums">{n.toLocaleString("pt-BR")}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
