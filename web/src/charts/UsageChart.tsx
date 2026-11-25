import { Area, AreaChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { axisTick, chartInk, limitColors, tooltipStyle } from "./colors";

const clock = (t: number) => new Date(t).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
const clockSec = (t: number) => new Date(t).toLocaleTimeString("pt-BR");

export type UsagePoint = { t: number; v: number };

type Props = {
  points: UsagePoint[];
  label: string; // nome da série, como "CPU"
  limits: { aviso: number; critico: number }; // limites dos alertas, na unidade do gráfico
  id: string; // identificador do gradiente, único na página
  unit?: "%" | "ms"; // percentual de 0 a 100 (padrão) ou milissegundos
};

// UsageChart desenha um percentual de uso entre 0 e 100%, ou um tempo em ms,
// com as linhas dos limites de alerta.
export default function UsageChart({ points, label, limits, id, unit = "%" }: Props) {
  const percent = unit === "%";
  const format = (v: number) => (percent ? `${v}%` : `${v} ms`);
  if (points.length === 0) {
    return <p className="flex h-44 items-center justify-center text-sm text-suave">Sem medições ainda.</p>;
  }
  const color = chartInk.series;
  return (
    <div role="img" aria-label={`${label} em ${percent ? "percentual" : "ms"}, ${points.length} pontos`} className="h-44">
      <ResponsiveContainer width="100%" height="100%" initialDimension={{ width: 600, height: 176 }}>
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
            // Em ms, o eixo vai até o maior valor ou até acima do limite crítico,
            // para as duas linhas de limite aparecerem.
            domain={percent ? [0, 100] : [0, (max: number) => Math.max(max, limits.critico * 1.1)]}
            ticks={percent ? [0, 25, 50, 75, 100] : undefined}
            tickFormatter={(v) => format(Number(v))}
            axisLine={false}
            tick={axisTick}
            tickLine={false}
            width={44}
          />
          <ReferenceLine y={limits.aviso} stroke={limitColors.aviso} strokeDasharray="4 4" strokeWidth={1} />
          <ReferenceLine y={limits.critico} stroke={limitColors.critico} strokeDasharray="4 4" strokeWidth={1} />
          <Tooltip
            cursor={{ stroke: chartInk.axis, strokeDasharray: "3 3" }}
            {...tooltipStyle}
            labelFormatter={(t) => clockSec(Number(t))}
            formatter={(v) => [`${Number(v).toLocaleString("pt-BR", { maximumFractionDigits: 1 })}${percent ? "%" : " ms"}`, label]}
          />
          <Area
            type="monotone"
            dataKey="v"
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
