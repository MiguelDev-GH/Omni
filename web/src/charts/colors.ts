// As cores dos gráficos são variáveis CSS (definidas em index.css, uma
// paleta por tema), então trocar de tema não exige redesenhar nada. A paleta
// categórica é validada para daltonismo na ordem abaixo. Cada protocolo tem
// cor fixa: a cor segue o protocolo, nunca a posição dele no ranking.
export const protocolColors: Record<string, string> = {
  HTTP: "var(--grafico-http)",
  PostgreSQL: "var(--grafico-postgresql)",
  HTTPS: "var(--grafico-https)",
  DNS: "var(--grafico-dns)",
  MySQL: "var(--grafico-mysql)",
  SMTP: "var(--grafico-smtp)",
  TCP: "var(--grafico-tcp)",
  UDP: "var(--grafico-udp)",
};

// otherColor é o cinza neutro de protocolos fora da lista.
export const otherColor = "var(--grafico-outro)";

export const protocolColor = (name: string) => protocolColors[name] ?? otherColor;

// Os gráficos ficam direto sobre o fundo: grade pontilhada clara, sem linha
// de eixo, e a área some em transparente para se fundir ao fundo.
export const chartInk = {
  axis: "var(--grafico-eixo)",
  grid: "var(--grafico-grade)",
  surface: "var(--color-elevado)",
  ground: "var(--color-fundo)",
  series: "var(--color-destaque)",
  text: "var(--color-texto)",
};

export const axisTick = { fontSize: 11, fill: chartInk.axis, fontFamily: "JetBrains Mono Variable, monospace" };

export const tooltipStyle = {
  contentStyle: {
    background: chartInk.surface,
    border: "none",
    borderRadius: 10,
    boxShadow: "var(--sombra-flutuante)",
    fontSize: 12,
    padding: "8px 10px",
  },
  labelStyle: { color: chartInk.text, fontFamily: "JetBrains Mono Variable, monospace", marginBottom: 2 },
  itemStyle: { color: "var(--color-suave)", padding: 0 },
};

// statusColors segue o significado da classe: sucesso, redirecionamento,
// erro do cliente e erro do servidor.
export const statusColors: Record<string, string> = {
  "2xx": "var(--grafico-2xx)",
  "3xx": "var(--grafico-3xx)",
  "4xx": "var(--grafico-4xx)",
  "5xx": "var(--grafico-5xx)",
};

// queryColor é a cor da série de consultas ao banco, a mesma do PostgreSQL.
export const queryColor = protocolColors.PostgreSQL;

// limitColors marca as linhas dos limites de alerta: aviso e crítico.
export const limitColors = { aviso: "var(--color-aviso)", critico: "var(--color-erro)" };
