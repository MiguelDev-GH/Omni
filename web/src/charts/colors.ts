// Paleta categórica para o fundo claro (#eef1f0), validada para daltonismo na
// ordem abaixo. Cada protocolo tem cor fixa: a cor segue o protocolo, nunca a
// posição dele no ranking.
export const protocolColors: Record<string, string> = {
  HTTP: "#2f74d0",
  PostgreSQL: "#c94f1d",
  HTTPS: "#0b8a76",
  DNS: "#a86d00",
  MySQL: "#c4416f",
  SMTP: "#2f7d16",
  TCP: "#6f5fd6",
  UDP: "#d24848",
};

// otherColor é o cinza neutro de protocolos fora da lista.
export const otherColor = "#93a09b";

export const protocolColor = (name: string) => protocolColors[name] ?? otherColor;

// Os gráficos ficam direto sobre o fundo: grade pontilhada clara, sem linha
// de eixo, e a área some em transparente para se fundir ao fundo.
export const chartInk = { axis: "#93a09b", grid: "#d3dad7", surface: "#ffffff", ground: "#eef1f0", series: "#0b8a76", text: "#16211e" };

export const axisTick = { fontSize: 11, fill: chartInk.axis, fontFamily: "JetBrains Mono Variable, monospace" };

export const tooltipStyle = {
  contentStyle: {
    background: chartInk.surface,
    border: "none",
    borderRadius: 10,
    boxShadow: "0 12px 32px -12px rgb(22 33 30 / 0.25), 0 0 0 1px rgb(22 33 30 / 0.06)",
    fontSize: 12,
    padding: "8px 10px",
  },
  labelStyle: { color: chartInk.text, fontFamily: "JetBrains Mono Variable, monospace", marginBottom: 2 },
  itemStyle: { color: "#5d6a66", padding: 0 },
};

// statusColors segue o significado da classe: sucesso, redirecionamento,
// erro do cliente e erro do servidor.
export const statusColors: Record<string, string> = {
  "2xx": "#0b8a76",
  "3xx": "#2f74d0",
  "4xx": "#c98a00",
  "5xx": "#d24848",
};

// queryColor é a cor da série de consultas ao banco, a mesma do PostgreSQL.
export const queryColor = "#c94f1d";

// limitColors marca as linhas dos limites de alerta: aviso e crítico.
export const limitColors = { aviso: "#c98a00", critico: "#d24848" };
