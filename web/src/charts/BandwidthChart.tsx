import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { formatBytes, type BandwidthPoint } from "../lib/traffic";
import { axisTick, chartInk, tooltipStyle } from "./colors";

const clock = (t: number) => new Date(t).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
const clockSec = (t: number) => new Date(t).toLocaleTimeString("pt-BR");

// BandwidthChart desenha os bytes por segundo dos últimos 15 minutos.
export default function BandwidthChart({ points }: { points: BandwidthPoint[] }) {
  if (points.length === 0) {
    return <p className="flex h-60 items-center justify-center text-sm text-suave">Sem tráfego capturado ainda.</p>;
  }
  return (
    <div role="img" aria-label={`Banda por segundo, ${points.length} pontos`} className="h-60">
      <ResponsiveContainer width="100%" height="100%" initialDimension={{ width: 600, height: 240 }}>
        <AreaChart data={points} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id="banda" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={chartInk.series} stopOpacity={0.22} />
              <stop offset="100%" stopColor={chartInk.series} stopOpacity={0} />
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
            tickFormatter={(v: number) => `${formatBytes(v)}/s`}
            axisLine={false}
            tick={axisTick}
            tickLine={false}
            width={80}
          />
          <Tooltip
            cursor={{ stroke: chartInk.axis, strokeDasharray: "3 3" }}
            {...tooltipStyle}
            labelFormatter={(t) => clockSec(Number(t))}
            formatter={(v) => [`${formatBytes(Number(v))}/s`, "Banda"]}
          />
          <Area
            type="monotone"
            dataKey="bytes"
            stroke={chartInk.series}
            strokeWidth={1.75}
            fill="url(#banda)"
            isAnimationActive={false}
            dot={false}
            activeDot={{ r: 4, stroke: chartInk.surface, strokeWidth: 2 }}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
