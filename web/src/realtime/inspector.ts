// Inspetor de pacotes: o comando INSPETOR liga e filtra o fluxo no servidor,
// e os lotes PACOTES chegam pelo mesmo WebSocket do resto do painel.

export type Packet = {
  hora: string;
  origem: string;
  destino: string;
  porta_origem: number;
  porta_destino: number;
  protocolo: string;
  tamanho: number;
  flags: string;
  info: string;
};

export type InspectorFilter = { ip: string; porta: number; protocolo: string };

export type PacketBatch = { pacotes: Packet[]; descartados: number };

export const emptyFilter: InspectorFilter = { ip: "", porta: 0, protocolo: "" };

// MAX_ROWS é o máximo de pacotes guardados na tabela; os mais antigos saem.
export const MAX_ROWS = 10_000;

type Command = { ativo: boolean; filtro: InspectorFilter };

let sender: ((msg: string) => void) | null = null;
let current: Command = { ativo: false, filtro: emptyFilter };
const listeners = new Set<(b: PacketBatch) => void>();

const encode = (c: Command) => JSON.stringify({ tipo: "INSPETOR", dados: c });

// bindInspectorSocket recebe a função de envio a cada conexão aberta (null ao
// fechar). Numa reconexão, a assinatura ativa é enviada de novo.
export function bindInspectorSocket(send: ((msg: string) => void) | null): void {
  sender = send;
  if (send && current.ativo) send(encode(current));
}

// setInspector liga ou desliga o fluxo de pacotes deste navegador.
export function setInspector(ativo: boolean, filtro: InspectorFilter = emptyFilter): void {
  current = { ativo, filtro };
  sender?.(encode(current));
}

// onPackets registra fn para cada lote recebido e devolve a função que a remove.
export function onPackets(fn: (b: PacketBatch) => void): () => void {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

function isPacket(v: unknown): v is Packet {
  return (
    isObject(v) &&
    typeof v.hora === "string" &&
    typeof v.origem === "string" &&
    typeof v.destino === "string" &&
    typeof v.porta_origem === "number" &&
    typeof v.porta_destino === "number" &&
    typeof v.protocolo === "string" &&
    typeof v.tamanho === "number" &&
    typeof v.flags === "string" &&
    typeof v.info === "string"
  );
}

// handlePackets entrega aos ouvintes um evento PACOTES; lotes malformados são ignorados.
export function handlePackets(dados: unknown): void {
  if (!isObject(dados) || !Array.isArray(dados.pacotes)) return;
  const batch = {
    pacotes: dados.pacotes.filter(isPacket),
    descartados: typeof dados.descartados === "number" ? dados.descartados : 0,
  };
  for (const fn of listeners) fn(batch);
}

// appendRows acrescenta next ao fim de rows, mantendo no máximo max linhas.
export function appendRows<T>(rows: T[], next: T[], max = MAX_ROWS): T[] {
  if (next.length === 0) return rows;
  const all = rows.concat(next);
  return all.length > max ? all.slice(all.length - max) : all;
}
