// O tema escolhido vive em localStorage, para valer entre abas e visitas; sem
// escolha guardada, segue a preferência do sistema. Sem armazenamento, a
// escolha vale só até recarregar. public/tema-inicial.js repete a leitura para
// aplicar o tema antes da primeira pintura.
export type Tema = "claro" | "escuro";

const KEY = "omni.tema";

export function initialTema(): Tema {
  try {
    const saved = localStorage.getItem(KEY);
    if (saved === "claro" || saved === "escuro") return saved;
  } catch {
    // sem armazenamento: segue o sistema
  }
  return typeof matchMedia === "function" && matchMedia("(prefers-color-scheme: dark)").matches ? "escuro" : "claro";
}

export function applyTema(t: Tema): void {
  document.documentElement.dataset.tema = t;
}

export function saveTema(t: Tema): void {
  try {
    localStorage.setItem(KEY, t);
  } catch {
    // sem armazenamento: a escolha dura até recarregar
  }
}
