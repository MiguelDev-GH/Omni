import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { axisTick, chartInk, tooltipStyle } from "./colors";

const clock = (t: number) => new Date(t).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
const clockSec = (t: number) => new Date(t).toLocaleTimeString("pt-BR");

export type CountPoint = { t: number; n: number };

type Props = {
  points: CountPoint[];
  label: string; // nome da série, como "Requisições"
  empty: string;
  color?: string;
  id: string; // identificador do gradiente, único na página
};

// CountChart desenha uma contagem por intervalo de 10 s, como requisições ou
// consultas, nos últimos 15 minutos.
export default function CountChart({ points, label, empty, color = chartInk.series, id }: Props) {
  if (points.length === 0) {
    return <p className="flex h-52 items-center justify-center text-sm text-suave">{empty}</p>;
  }
  return (
    <div role="img" aria-label={`${label} a cada 10 s, ${points.length} pontos`} className="h-52">
      <ResponsiveContainer width="100%" height="100%" initialDimension={{ width: 600, height: 208 }}>
        <AreaChart data={points} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.22} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke={chartInk.grid} strokeDasharray="1 5" vertical={false} />
          <XAxis
            dataKey="t"
            type="number"
            domain={["dataMin", "dataMax"]}
            scale="time"
            tickFormatter={clock}
            axisLine={false}
            tick={axisTick}
            tickLine={false}
            minTickGap={48}
          />
          <YAxis
            allowDecimals={false}
            axisLine={false}
            tick={axisTick}
            tickLine={false}
            width={48}
          />
          <Tooltip
            cursor={{ stroke: chartInk.axis, strokeDasharray: "3 3" }}
            {...tooltipStyle}
            labelFormatter={(t) => clockSec(Number(t))}
            formatter={(v) => [Number(v).toLocaleString("pt-BR"), label]}
          />
          <Area
            type="monotone"
            dataKey="n"
            stroke={color}
            strokeWidth={1.75}
            fill={`url(#${id})`}
            isAnimationActive={false}
            dot={false}
            activeDot={{ r: 4, stroke: chartInk.surface, strokeWidth: 2 }}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
