import { useVirtualizer } from "@tanstack/react-virtual";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Notice, PageHeader, Stat } from "../components/Card";
import Select from "../components/Select";
import {
  appendRows,
  emptyFilter,
  linkAcks,
  MAX_ROWS,
  onPackets,
  pruneAcks,
  setInspector,
  type AckIndex,
  type InspectorFilter,
  type Packet,
} from "../realtime/inspector";
import { useAppStore } from "../store/app";

// confirma é o id do segmento que este ACK confirma, quando ele está na tabela.
type Row = Packet & { id: number; confirma?: number };

const protocols = ["HTTP", "HTTPS", "PostgreSQL", "MySQL", "DNS", "SMTP", "TCP", "UDP", "ICMP"];

const protocolOptions = [{ value: "", label: "Todos os protocolos" }, ...protocols.map((p) => ({ value: p, label: p }))];

const ROW_HEIGHT = 32;

// As colunas ficam numa grade, porque a lista virtualizada não é uma <table>.
const columns = "grid grid-cols-[3.5rem_6.5rem_minmax(9rem,1fr)_minmax(9rem,1fr)_5.5rem_4.5rem_minmax(10rem,1.5fr)] gap-x-3";

const time = (iso: string) => {
  const d = new Date(iso);
  const pad = (n: number, w = 2) => String(n).padStart(w, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
};

const endpoint = (ip: string, port: number) => (port ? (ip.includes(":") ? `[${ip}]:${port}` : `${ip}:${port}`) : ip);

const count = (n: number) => n.toLocaleString("pt-BR");

const number = (id: number) => `Nº ${count(id + 1)}`;

// info mostra, num ACK sem conteúdo próprio, qual pacote ele confirma.
const info = (r: Row) =>
  r.info || (r.confirma !== undefined ? `${r.flags} · confirma ${number(r.confirma)}` : r.flags);

// validFilter confere o formulário e devolve o filtro ou a mensagem de erro.
function validFilter(ip: string, port: string, protocolo: string): InspectorFilter | string {
  const p = port.trim() === "" ? 0 : Number(port);
  if (!Number.isInteger(p) || p < 0 || p > 65535) return "A porta precisa ser um número de 1 a 65535.";
  const addr = ip.trim();
  if (addr !== "" && !/^[0-9a-fA-F.:]+$/.test(addr)) return "O IP precisa ser um endereço IPv4 ou IPv6.";
  return { ip: addr, porta: p, protocolo };
}

export default function InspectorPage() {
  const sniffer = useAppStore((s) => s.engines.sniffer);
  const [rows, setRows] = useState<Row[]>([]);
  const [dropped, setDropped] = useState(0);
  const [paused, setPaused] = useState(false);
  const [filter, setFilter] = useState<InspectorFilter>(emptyFilter);
  const [ipField, setIpField] = useState("");
  const [portField, setPortField] = useState("");
  const [protoField, setProtoField] = useState("");
  const [filterError, setFilterError] = useState("");
  const [selected, setSelected] = useState<Row | null>(null);
  const nextId = useRef(0);
  const acks = useRef<AckIndex>(new Map());
  const scroller = useRef<HTMLDivElement>(null);

  // Assina enquanto a página está aberta e não pausada; sair da página ou
  // pausar manda ativo:false.
  useEffect(() => {
    if (paused) return;
    setInspector(true, filter);
    const off = onPackets((b) => {
      const batch = linkAcks(
        acks.current,
        b.pacotes.map((p) => ({ ...p, id: nextId.current++ })),
      );
      setRows((prev) => {
        const next = appendRows(prev, batch);
        if (acks.current.size > 2 * MAX_ROWS) pruneAcks(acks.current, next[0]!.id);
        return next;
      });
      if (b.descartados) setDropped((d) => d + b.descartados);
    });
    return () => {
      off();
      setInspector(false, filter);
    };
  }, [paused, filter]);

  // O React Compiler não memoiza este componente por causa do virtualizador,
  // que devolve funções com estado próprio; a página re-renderiza a cada lote.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scroller.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
    // A altura da área de rolagem (h-[32rem]), usada até a primeira medição.
    initialRect: { width: 0, height: 512 },
  });

  function applyFilter(e: FormEvent) {
    e.preventDefault();
    const f = validFilter(ipField, portField, protoField);
    if (typeof f === "string") {
      setFilterError(f);
      return;
    }
    setFilterError("");
    acks.current.clear();
    setRows([]);
    setSelected(null);
    setFilter(f);
  }

  // rowById acha a linha pelo id: os ids da tabela são contíguos.
  function rowById(id: number): { row: Row; index: number } | null {
    const index = rows.length ? id - rows[0]!.id : -1;
    const row = rows[index];
    return row?.id === id ? { row, index } : null;
  }

  function jumpTo(id: number) {
    const found = rowById(id);
    if (!found) return;
    setSelected(found.row);
    virtual.scrollToIndex(found.index, { align: "center" });
  }

  function clear() {
    acks.current.clear();
    setRows([]);
    setDropped(0);
    setSelected(null);
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Inspetor de pacotes"
        aside={
          <div className="flex gap-2">
            <button type="button" className="botao-leve" onClick={clear}>
              Limpar
            </button>
            <button type="button" className="botao" aria-pressed={paused} onClick={() => setPaused((p) => !p)}>
              {paused ? "Retomar" : "Pausar"}
            </button>
          </div>
        }
      >
        Os pacotes das portas dos alvos, ao vivo, um por linha. No HTTP aparecem só o método e a rota, e no PostgreSQL só o
        tipo da mensagem: cabeçalhos, corpos e consultas nunca saem do servidor. A tabela guarda os últimos{" "}
        {count(MAX_ROWS)} pacotes.
      </PageHeader>

      {sniffer !== "ativo" && (
        <Notice>
          {sniffer === "sem-permissao"
            ? "O sniffer está sem permissão de captura, então nenhum pacote vai chegar. Veja no README como liberar a captura."
            : "O sniffer não está capturando agora. Cadastre um alvo com portas para ele começar."}
        </Notice>
      )}

      <form onSubmit={applyFilter} aria-label="Filtros" className="flex flex-wrap items-end gap-3">
        <div className="space-y-1.5">
          <label htmlFor="filtro-ip" className="block text-xs font-medium text-suave">
            IP
          </label>
          <input
            id="filtro-ip"
            className="campo w-48 font-mono"
            placeholder="qualquer"
            value={ipField}
            onChange={(e) => setIpField(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <label htmlFor="filtro-porta" className="block text-xs font-medium text-suave">
            Porta
          </label>
          <input
            id="filtro-porta"
            inputMode="numeric"
            className="campo w-28 font-mono"
            placeholder="qualquer"
            value={portField}
            onChange={(e) => setPortField(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <span className="block text-xs font-medium text-suave">Protocolo</span>
          <Select label="Protocolo" value={protoField} options={protocolOptions} onChange={setProtoField} />
        </div>
        <button type="submit" className="botao">
          Filtrar
        </button>
        {filterError && (
          <p role="alert" className="w-full text-sm text-erro">
            {filterError}
          </p>
        )}
      </form>

      <div className="grid grid-cols-3 gap-4 sm:gap-6">
        <Stat label="Na tabela" value={count(rows.length)} />
        <Stat label="Descartados" value={count(dropped)} tone={dropped > 0 ? "text-aviso" : undefined} />
        <Stat label="Fluxo" value={paused ? "pausado" : "ao vivo"} tone={paused ? "text-suave" : "text-destaque"} />
      </div>

      <div className="grid gap-x-8 gap-y-6 xl:grid-cols-[minmax(0,1fr)_18rem]">
        <div className="min-w-0 overflow-x-auto">
          <div role="table" aria-label="Pacotes" aria-rowcount={rows.length} className="min-w-[46rem] text-sm">
            <div role="row" className={`${columns} pb-2 text-xs font-medium text-suave`}>
              {["Nº", "Hora", "Origem", "Destino", "Protocolo", "Tamanho", "Info"].map((h) => (
                <span key={h} role="columnheader" className={h === "Tamanho" ? "text-right" : ""}>
                  {h}
                </span>
              ))}
            </div>
            {rows.length === 0 ? (
              <p className="border-t border-borda/70 py-6 text-sm text-suave">
                {paused ? "Pausado. Retome para ver os pacotes novos." : "Esperando pacotes…"}
              </p>
            ) : (
              <div ref={scroller} className="h-[32rem] overflow-y-auto border-t border-borda/70">
                <div role="rowgroup" className="relative" style={{ height: virtual.getTotalSize() }}>
                  {virtual.getVirtualItems().map((v) => {
                    const r = rows[v.index]!;
                    const active = selected?.id === r.id;
                    const confirmed = selected?.confirma === r.id;
                    return (
                      <div
                        key={r.id}
                        role="row"
                        aria-rowindex={v.index + 1}
                        aria-selected={active}
                        tabIndex={0}
                        onClick={() => setSelected(r)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            setSelected(r);
                          }
                        }}
                        className={`${columns} absolute inset-x-0 cursor-pointer items-center border-b border-borda/40 font-mono text-xs ${
                          active ? "bg-elevado" : confirmed ? "bg-destaque/10" : "hover:bg-elevado/60"
                        }`}
                        style={{ height: ROW_HEIGHT, transform: `translateY(${v.start}px)` }}
                      >
                        <span role="cell" className="text-apagado tabular-nums">
                          {count(r.id + 1)}
                        </span>
                        <span role="cell" className="text-suave tabular-nums">
                          {time(r.hora)}
                        </span>
                        <span role="cell" className="truncate">
                          {endpoint(r.origem, r.porta_origem)}
                        </span>
                        <span role="cell" className="truncate">
                          {endpoint(r.destino, r.porta_destino)}
                        </span>
                        <span role="cell" className="text-destaque-forte">
                          {r.protocolo}
                        </span>
                        <span role="cell" className="text-right tabular-nums">
                          {count(r.tamanho)}
                        </span>
                        <span role="cell" className="truncate text-suave">
                          {info(r)}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </div>

        <aside aria-label="Detalhe do pacote" className="space-y-3">
          <h2 className="text-sm font-medium text-texto">Detalhe do pacote</h2>
          {selected ? (
            <dl className="space-y-2 text-sm">
              {[
                ["Nº", count(selected.id + 1)],
                ["Hora", time(selected.hora)],
                ["Origem", endpoint(selected.origem, selected.porta_origem)],
                ["Destino", endpoint(selected.destino, selected.porta_destino)],
                ["Protocolo", selected.protocolo],
                ["Tamanho", `${count(selected.tamanho)} bytes`],
                ["Flags TCP", selected.flags || "—"],
                ["Info", selected.info || "—"],
                ...(selected.flags
                  ? [
                      ["Seq", String(selected.seq)],
                      ["Ack", hasAck(selected) ? String(selected.ack) : "—"],
                      ["Carga", `${count(selected.carga)} bytes`],
                    ]
                  : []),
              ].map(([k, v]) => (
                <div key={k} className="grid grid-cols-[6rem_1fr] gap-2">
                  <dt className="text-suave">{k}</dt>
                  <dd className="font-mono text-xs break-all">{v}</dd>
                </div>
              ))}
              {hasAck(selected) && (
                <div className="grid grid-cols-[6rem_1fr] gap-2">
                  <dt className="text-suave">Confirma</dt>
                  <dd className="font-mono text-xs">
                    <Confirms row={selected} target={selected.confirma} lookup={rowById} onJump={jumpTo} />
                  </dd>
                </div>
              )}
            </dl>
          ) : (
            <p className="text-sm text-suave">Clique numa linha para ver o pacote aqui.</p>
          )}
        </aside>
      </div>
    </div>
  );
}

const hasAck = (r: Packet) => r.flags.split(",").includes("ACK");

// Confirms diz qual segmento o ACK confirma e leva até ele na tabela.
function Confirms({
  row,
  target,
  lookup,
  onJump,
}: {
  row: Row;
  target: number | undefined;
  lookup: (id: number) => { row: Row } | null;
  onJump: (id: number) => void;
}) {
  if (target === undefined)
    return <span className="text-suave">Nenhum segmento da tabela bate com o Ack {row.ack}.</span>;
  const found = lookup(target);
  if (!found) return <span className="text-suave">{number(target)}, que já saiu da tabela.</span>;
  return (
    <button type="button" className="text-left text-destaque hover:underline" onClick={() => onJump(target)}>
      {number(target)} · {found.row.info || found.row.flags}
      <span className="block text-suave">ACK cumulativo: confirma também o que veio antes</span>
    </button>
  );
}
