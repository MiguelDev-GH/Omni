// Aplica o tema antes da primeira pintura, para a página não piscar clara
// quando o tema é escuro. É um arquivo à parte porque a CSP não aceita script
// inline. A mesma regra vive em src/lib/tema.ts.
try {
  var tema = localStorage.getItem("omni.tema");
  if (tema !== "claro" && tema !== "escuro") {
    tema = matchMedia("(prefers-color-scheme: dark)").matches ? "escuro" : "claro";
  }
  document.documentElement.dataset.tema = tema;
} catch {
  // sem armazenamento ou sem matchMedia: fica o tema claro
}
