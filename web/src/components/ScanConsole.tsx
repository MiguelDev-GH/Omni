import { useEffect, useRef } from "react";
import { useAppStore } from "../store/app";

const EMPTY: string[] = [];

// ScanConsole mostra a saída ao vivo de uma varredura, rolando até a última
// linha enquanto o usuário não sobe a rolagem.
export default function ScanConsole({ scanId }: { scanId: number | null }) {
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
      className="h-72 overflow-auto rounded-lg border border-borda bg-black p-3 font-mono text-xs leading-5 text-zinc-300"
    >
      {scanId === null
        ? "Nenhuma varredura iniciada nesta sessão."
        : lines.length === 0
          ? "Aguardando a saída da ferramenta..."
          : lines.join("\n")}
    </pre>
  );
}
