import { useState, type FormEvent } from "react";
import { verifyToken } from "../api/client";
import { useAppStore } from "../store/app";

const MIN_TOKEN_LEN = 32;

const messages = {
  invalido: "Token inválido.",
  "sem-conexao": "Não foi possível falar com o Omni. Ele está rodando?",
} as const;

export default function TokenPage() {
  const setToken = useAppStore((s) => s.setToken);
  const expired = useAppStore((s) => s.sessionExpired);
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

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
    <main className="flex min-h-screen items-center justify-center px-4">
      <form onSubmit={onSubmit} className="w-full max-w-sm animate-entrada space-y-6 rounded-2xl bg-superficie/80 p-8 backdrop-blur-sm">
        <div className="space-y-1">
          <h1 className="flex items-baseline font-mono text-2xl font-semibold tracking-tight">
            omni
            <span aria-hidden="true" className="ml-0.5 inline-block h-5 w-2.5 translate-y-0.5 animate-cursor bg-destaque" />
          </h1>
          <p className="text-sm text-suave">Informe o token definido em <code className="font-mono text-xs">OMNI_TOKEN</code> ao iniciar o Omni.</p>
        </div>
        {expired && (
          <p role="status" className="text-sm text-aviso">
            Sessão expirada. Informe o token novamente.
          </p>
        )}
        <div className="space-y-1.5">
          <label htmlFor="token" className="block text-xs font-medium text-suave">
            Token de acesso
          </label>
          <input
            id="token"
            type="password"
            autoComplete="current-password"
            autoFocus
            value={value}
            onChange={(e) => setValue(e.target.value)}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? "token-erro" : undefined}
            className="campo font-mono"
          />
          {error && (
            <p id="token-erro" className="text-sm text-erro">
              {error}
            </p>
          )}
        </div>
        <button
          type="submit"
          disabled={busy}
          className="botao w-full py-2.5"
        >
          Entrar
        </button>
      </form>
    </main>
  );
}
