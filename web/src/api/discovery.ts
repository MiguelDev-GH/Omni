import { apiFetch } from "./client";

export type Suggestion = {
  processo?: string;
  pid?: number;
  host: string;
  tipo: "app" | "banco";
  portas: number[];
};

export function listSuggestions(): Promise<Suggestion[]> {
  return apiFetch<Suggestion[]>("/api/descoberta");
}
