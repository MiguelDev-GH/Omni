import { useAppStore } from "../store/app";

export class ApiError extends Error {
  status: number;
  field?: string;

  constructor(status: number, message: string, field?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.field = field;
  }
}

// apiFetch chama a API com o token da sessão. Um 401 apaga o token, o que
// leva a interface de volta à tela de token.
export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  const token = useAppStore.getState().token;
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (init.body !== undefined && init.body !== null) headers.set("Content-Type", "application/json");

  const resp = await fetch(path, { ...init, headers });
  if (resp.status === 401) {
    useAppStore.getState().clearToken(true);
    throw new ApiError(401, "sessão expirada");
  }
  if (!resp.ok) {
    const body = await resp.json().catch(() => null);
    if (body && typeof body.erro === "string") {
      throw new ApiError(resp.status, body.erro, typeof body.campo === "string" ? body.campo : undefined);
    }
    throw new ApiError(resp.status, `erro HTTP ${resp.status}`);
  }
  if (resp.status === 204) return undefined as T;
  return (await resp.json()) as T;
}

export type TokenCheck = "ok" | "invalido" | "sem-conexao";

// verifyToken testa um token candidato sem gravá-lo no store.
export async function verifyToken(t: string): Promise<TokenCheck> {
  try {
    const resp = await fetch("/api/alvos", { headers: { Authorization: `Bearer ${t}` } });
    if (resp.ok) return "ok";
    if (resp.status === 401) return "invalido";
    return "sem-conexao";
  } catch {
    return "sem-conexao";
  }
}
