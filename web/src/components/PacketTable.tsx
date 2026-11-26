import { useVirtualizer } from "@tanstack/react-virtual";
import { useRef } from "react";
import type { Packet } from "../realtime/inspector";

// confirma é o id do segmento que este ACK confirma, quando ele está na tabela.
export type PacketRow = Packet & { id: number; confirma?: number };
type Row = PacketRow;

const ROW_HEIGHT = 32;

// As colunas ficam numa grade, porque a lista virtualizada não é uma <table>.
const columns =
  "grid grid-cols-[3.5rem_6.5rem_minmax(9rem,1fr)_minmax(9rem,1fr)_5.5rem_4.5rem_minmax(10rem,1.5fr)] gap-x-3";

const time = (iso: string) => {
  const d = new Date(iso);
  const pad = (n: number, w = 2) => String(n).padStart(w, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
};

const endpoint = (ip: string, port: number) => (port ? (ip.includes(":") ? `[${ip}]:${port}` : `${ip}:${port}`) : ip);

export const count = (n: number) => n.toLocaleString("pt-BR");

const number = (id: number) => `Nº ${count(id + 1)}`;

// info mostra, num ACK sem conteúdo próprio, qual pacote ele confirma.
const info = (r: Row) => r.info || (r.confirma !== undefined ? `${r.flags} · confirma ${number(r.confirma)}` : r.flags);

// PacketTable é a tabela do Inspetor, virtualizada, com o detalhe do pacote
// selecionado ao lado. Os ids das linhas precisam ser contíguos.
export default function PacketTable({
  rows,
  empty,
  selected,
  onSelect,
}: {
  rows: Row[];
  empty: string;
  selected: Row | null;
  onSelect: (r: Row) => void;
}) {
  const scroller = useRef<HTMLDivElement>(null);

  // O React Compiler não memoiza este componente por causa do virtualizador,
  // que devolve funções com estado próprio; a tabela re-renderiza a cada lote.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scroller.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
    // A altura da área de rolagem (h-[32rem]), usada até a primeira medição.
    initialRect: { width: 0, height: 512 },
  });

  // rowById acha a linha pelo id: os ids da tabela são contíguos.
  function rowById(id: number): { row: Row; index: number } | null {
    const index = rows.length ? id - rows[0]!.id : -1;
    const row = rows[index];
    return row?.id === id ? { row, index } : null;
  }

  function jumpTo(id: number) {
    const found = rowById(id);
    if (!found) return;
    onSelect(found.row);
    virtual.scrollToIndex(found.index, { align: "center" });
  }

  return (
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
            <p className="border-t border-borda/70 py-6 text-sm text-suave">{empty}</p>
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
                      onClick={() => onSelect(r)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          onSelect(r);
                        }
                      }}
                      className={`${columns} absolute inset-x-0 cursor-pointer items-center border-b border-borda/40 font-mono text-xs ${
                        active ? "bg-elevado" : confirmed ? "bg-destaque/10" : "hover:bg-elevado/60"
                      }`}
                      style={{
                        height: ROW_HEIGHT,
                        transform: `translateY(${v.start}px)`,
                      }}
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
