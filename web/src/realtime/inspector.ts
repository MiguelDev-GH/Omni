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
  // No TCP: número de sequência, de confirmação e tamanho da carga em bytes.
  seq: number;
  ack: number;
  carga: number;
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
    typeof v.info === "string" &&
    typeof v.seq === "number" &&
    typeof v.ack === "number" &&
    typeof v.carga === "number"
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

// AckIndex guarda, por sentido da conexão e fim do segmento no espaço de
// sequência, o id do último pacote que termina ali.
export type AckIndex = Map<string, number>;

const hasFlag = (p: Packet, f: string) => p.flags.split(",").includes(f);

const flowKey = (ip: string, port: number, toIp: string, toPort: number, seq: number) =>
  `${ip}|${port}|${toIp}|${toPort}|${seq}`;

// seqEnd devolve o número que o ACK do segmento vai trazer, ou null se o
// pacote não ocupa espaço de sequência (ACK puro), já que ninguém o confirma.
function seqEnd(p: Packet): number | null {
  const used = p.carga + (hasFlag(p, "SYN") ? 1 : 0) + (hasFlag(p, "FIN") ? 1 : 0);
  return used > 0 ? (p.seq + used) >>> 0 : null;
}

// linkAcks marca em confirma o id do segmento que cada ACK confirma: o último
// pacote do sentido oposto que termina no número do ACK. Como o ACK do TCP é
// cumulativo, ele também confirma os segmentos anteriores a esse.
export function linkAcks<T extends Packet & { id: number }>(index: AckIndex, rows: T[]): (T & { confirma?: number })[] {
  return rows.map((r) => {
    if (r.flags === "") return r;
    let confirma: number | undefined;
    if (hasFlag(r, "ACK")) confirma = index.get(flowKey(r.destino, r.porta_destino, r.origem, r.porta_origem, r.ack));
    const end = seqEnd(r);
    if (end !== null) index.set(flowKey(r.origem, r.porta_origem, r.destino, r.porta_destino, end), r.id);
    return confirma === undefined ? r : { ...r, confirma };
  });
}

// pruneAcks esquece os segmentos com id menor que minId, que já saíram da tabela.
export function pruneAcks(index: AckIndex, minId: number): void {
  for (const [k, id] of index) if (id < minId) index.delete(k);
}
