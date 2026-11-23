import { useMemo, useState, type CSSProperties, type FormEvent } from "react";
import { verifyToken } from "../api/client";
import logo from "../assets/logo.png";
import { useAppStore } from "../store/app";

const MIN_TOKEN_LEN = 32;

const messages = {
  invalido: "Token inválido.",
  "sem-conexao": "Não foi possível falar com o Omni. Ele está rodando?",
} as const;

const watches = [
  "Tráfego e protocolos da máquina",
  "Requisições da API e consultas ao banco",
  "Alertas por limite e varreduras do Arsenal",
];

// delay escalona a entrada de cada bloco, numa única sequência de abertura.
const delay = (ms: number): CSSProperties => ({ animationDelay: `${ms}ms` });

// signalPath desenha uma linha de sinal aleatória, como um traço de tráfego,
// que atravessa a parte de baixo da tela. O nível passeia ao acaso e, de vez
// em quando, dá um pico de altura e direção variadas; a cada abertura o
// desenho é outro.
function signalPath(width: number, height: number): string {
  const mid = height * 0.6;
  let level = mid;
  const points: string[] = [];
  for (let x = 0; x <= width; x += 6 + Math.random() * 10) {
    level += (Math.random() - 0.5) * height * 0.2;
    level += (mid - level) * 0.06; // puxa de volta ao meio para não sair da faixa
    let y = level;
    if (Math.random() < 0.07) y -= (0.15 + Math.random() * 0.4) * height * (Math.random() < 0.8 ? 1 : -0.5);
    y = Math.min(Math.max(y, 4), height - 4);
    points.push(`${x.toFixed(0)},${y.toFixed(1)}`);
  }
  return `M${points.join(" L")}`;
}

export default function TokenPage() {
  const setToken = useAppStore((s) => s.setToken);
  const expired = useAppStore((s) => s.sessionExpired);
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const path = useMemo(() => signalPath(1600, 120), []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const t = value.trim();
    if (t.length < MIN_TOKEN_LEN) {
      setError(`O token tem pelo menos ${MIN_TOKEN_LEN} caracteres.`);
      return;
    }
    setBusy(true);
    const result = await verifyToken(t);
    setBusy(false);
    if (result === "ok") setToken(t);
    else setError(messages[result]);
  }

  return (
    <main className="relative flex min-h-screen flex-col overflow-hidden">
      <div className="mx-auto grid w-full max-w-6xl flex-1 content-center gap-16 px-6 py-16 md:grid-cols-[1.25fr_1fr] md:items-end md:gap-24 md:px-12">
        <section>
          <h1 className="animate-entrada">
            <img src={logo} alt="Omni" className="h-20 w-auto md:h-28" />
          </h1>
          <ul className="mt-8 space-y-2.5 text-sm">
            {watches.map((w, i) => (
              <li key={w} className="flex animate-entrada items-center gap-3" style={delay(160 + i * 80)}>
                <span aria-hidden="true" className="h-px w-5 bg-destaque" />
                {w}
              </li>
            ))}
          </ul>
        </section>

        <form onSubmit={onSubmit} noValidate className="w-full max-w-sm animate-entrada space-y-8" style={delay(420)}>
          {expired && (
            <p role="status" className="flex items-center gap-2 text-sm text-aviso">
              <span aria-hidden="true" className="size-1.5 rounded-full bg-aviso" />
              Sessão expirada. Informe o token novamente.
            </p>
          )}
          <div>
            <label htmlFor="token" className="block text-xs font-medium text-suave">
              Token de acesso
            </label>
            <div className="relative">
              <input
                id="token"
                type="password"
                autoComplete="current-password"
                autoFocus
                spellCheck={false}
                value={value}
                onChange={(e) => {
                  setValue(e.target.value);
                  if (error) setError(null);
                }}
                aria-invalid={error ? true : undefined}
                aria-describedby={error ? "token-erro" : undefined}
                className="peer w-full border-0 border-b border-borda bg-transparent py-3 font-mono text-lg tracking-wider outline-none placeholder:text-apagado focus-visible:outline-none aria-invalid:border-erro/60"
                placeholder="••••••••••••••••"
              />
              {/* Traço que cresce da esquerda quando o campo recebe foco. */}
              <span
                aria-hidden="true"
                className="absolute -bottom-px left-0 h-0.5 w-full origin-left scale-x-0 bg-destaque transition-transform duration-500 ease-(--ease-suave) peer-focus:scale-x-100 peer-aria-invalid:bg-erro"
              />
            </div>
            {error && (
              <p id="token-erro" className="mt-2 animate-entrada text-sm text-erro">
                {error}
              </p>
            )}
          </div>
          <button type="submit" disabled={busy} aria-busy={busy} className="botao w-full py-3">
            {busy && <span aria-hidden="true" className="size-1.5 animate-pulso rounded-full bg-fundo text-fundo" />}
            Entrar
          </button>
        </form>
      </div>

      {/* Linha de sinal: desenhada uma vez na abertura e depois parada. */}
      <svg
        aria-hidden="true"
        viewBox="0 0 1600 120"
        preserveAspectRatio="none"
        className="pointer-events-none h-24 w-full text-destaque md:h-32"
      >
        <path
          d={path}
          pathLength={1}
          fill="none"
          stroke="currentColor"
          strokeOpacity={0.35}
          strokeWidth={1.25}
          vectorEffect="non-scaling-stroke"
          className="animate-desenho [stroke-dasharray:1]"
        />
      </svg>
      <footer
        className="flex animate-entrada justify-between px-6 pb-6 font-mono text-[11px] text-apagado md:px-12"
        style={delay(600)}
      >
        <span>{window.location.host}</span>
        <span>acesso local</span>
      </footer>
    </main>
  );
}
