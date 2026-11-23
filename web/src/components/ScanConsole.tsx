import { useEffect, useRef } from "react";
import { useAppStore } from "../store/app";

const EMPTY: string[] = [];

// ScanConsole mostra a saída ao vivo de uma varredura, rolando até a última
// linha enquanto o usuário não sobe a rolagem.
// finished indica uma varredura já terminada, cuja saída pode não ter sido
// acompanhada nesta sessão.
export default function ScanConsole({ scanId, finished = false }: { scanId: number | null; finished?: boolean }) {
  const lines = useAppStore((s) => (scanId === null ? EMPTY : (s.scanLogs[scanId] ?? EMPTY)));
  const ref = useRef<HTMLPreElement>(null);
  const stick = useRef(true);

  useEffect(() => {
    const el = ref.current;
    if (el && stick.current) el.scrollTop = el.scrollHeight;
  }, [lines]);

  return (
    <pre
      ref={ref}
      role="log"
      aria-label="Saída da varredura"
      onScroll={(e) => {
        const el = e.currentTarget;
        stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
      }}
      className="h-72 overflow-auto rounded-2xl bg-console p-4 font-mono text-xs leading-5 text-console-texto dark:ring-1 dark:ring-borda selection:bg-destaque/40 selection:text-white"
    >
      {scanId === null
        ? "Nenhuma varredura iniciada nesta sessão."
        : lines.length === 0
          ? finished
            ? "A saída ao vivo só aparece para varreduras acompanhadas nesta aba."
            : "Aguardando a saída da ferramenta..."
          : lines.join("\n")}
    </pre>
  );
}
