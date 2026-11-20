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
      <form onSubmit={onSubmit} className="w-full max-w-sm space-y-4 rounded-lg border border-borda bg-superficie p-6">
        <h1 className="text-2xl font-semibold">Omni</h1>
        {expired && (
          <p role="status" className="text-sm text-amber-400">
            Sessão expirada. Informe o token novamente.
          </p>
        )}
        <div className="space-y-1">
          <label htmlFor="token" className="block text-sm text-suave">
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
            className="w-full rounded border border-borda bg-fundo px-3 py-2 outline-none focus:border-destaque"
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
          className="w-full rounded bg-destaque px-3 py-2 font-medium text-zinc-950 disabled:opacity-60"
        >
          Entrar
        </button>
      </form>
    </main>
  );
}
