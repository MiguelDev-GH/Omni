// O token vive só na aba (sessionStorage). Sem armazenamento disponível,
// o acesso falha em silêncio e o token fica apenas em memória.
const KEY = "omni.token";

export function readToken(): string | null {
  try {
    return sessionStorage.getItem(KEY);
  } catch {
    return null;
  }
}

export function writeToken(t: string): void {
  try {
    sessionStorage.setItem(KEY, t);
  } catch {
    // sem armazenamento: segue em memória
  }
}

export function removeToken(): void {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    // sem armazenamento: nada a remover
  }
}
