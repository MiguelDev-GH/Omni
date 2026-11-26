import { useAppStore } from "../store/app";
import type { Packet } from "../realtime/inspector";
import { ApiError, apiFetch } from "./client";

// Capture é uma gravação .pcap, como o backend devolve (internal/capture).
export type Capture = {
  id: number;
  inicio: string;
  fim?: string;
  duracao_s: number;
  // porta 0 grava todas as portas dos alvos.
  porta: number;
  pacotes: number;
  bytes: number;
  descartados: number;
  status: "gravando" | "concluida" | "falhou";
  erro?: string;
};

export type CapturePage = { pacotes: Packet[]; pagina: number; por_pagina: number; total: number };

export const MIN_DURATION = 10;
export const MAX_DURATION = 300;

export function listCaptures(): Promise<Capture[]> {
  return apiFetch<Capture[]>("/api/capturas");
}

export function startCapture(duracao_s: number, porta: number): Promise<{ id: number }> {
  return apiFetch<{ id: number }>("/api/capturas", { method: "POST", body: JSON.stringify({ duracao_s, porta }) });
}

export function deleteCapture(id: number): Promise<void> {
  return apiFetch<void>(`/api/capturas/${id}`, { method: "DELETE" });
}

export function capturePackets(id: number, pagina: number): Promise<CapturePage> {
  return apiFetch<CapturePage>(`/api/capturas/${id}/pacotes?pagina=${pagina}`);
}

// downloadCapture busca o .pcap com o token e o salva pelo navegador: um link
// direto para a API não levaria o cabeçalho Authorization.
export async function downloadCapture(id: number): Promise<void> {
  const token = useAppStore.getState().token;
  const resp = await fetch(`/api/capturas/${id}/arquivo`, { headers: token ? { Authorization: `Bearer ${token}` } : {} });
  if (resp.status === 401) {
    useAppStore.getState().clearToken(true);
    throw new ApiError(401, "sessão expirada");
  }
  if (!resp.ok) {
    const body = await resp.json().catch(() => null);
    throw new ApiError(resp.status, body && typeof body.erro === "string" ? body.erro : `erro HTTP ${resp.status}`);
  }
  const url = URL.createObjectURL(await resp.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = `omni-captura-${id}.pcap`;
  document.body.append(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
