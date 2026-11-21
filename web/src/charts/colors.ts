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
