// Paleta categórica para a superfície escura (zinc-900), validada para
// daltonismo na ordem abaixo. Cada protocolo tem cor fixa: a cor segue o
// protocolo, nunca a posição dele no ranking.
export const protocolColors: Record<string, string> = {
  HTTP: "#3987e5",
  PostgreSQL: "#d95926",
  HTTPS: "#199e70",
  DNS: "#c98500",
  MySQL: "#d55181",
  SMTP: "#008300",
  TCP: "#9085e9",
  UDP: "#e66767",
};

// otherColor é o cinza neutro de protocolos fora da lista.
export const otherColor = "#71717a";

export const protocolColor = (name: string) => protocolColors[name] ?? otherColor;

export const chartInk = { axis: "#a1a1aa", grid: "#27272a", surface: "#18181b", series: "#3987e5" };

// statusColors segue o significado da classe: sucesso, redirecionamento,
// erro do cliente e erro do servidor.
export const statusColors: Record<string, string> = {
  "2xx": "#199e70",
  "3xx": "#3987e5",
  "4xx": "#c98500",
  "5xx": "#e66767",
};

// queryColor é a cor da série de consultas ao banco, a mesma do PostgreSQL.
export const queryColor = "#d95926";
