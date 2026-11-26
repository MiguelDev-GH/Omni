import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router";
import {
  deleteCapture,
  downloadCapture,
  listCaptures,
  MAX_DURATION,
  MIN_DURATION,
  startCapture,
  type Capture,
} from "../api/captures";
import { ApiError } from "../api/client";
import { targetPorts } from "../api/targets";
import { Notice, PageHeader } from "../components/Card";
import Select from "../components/Select";
import { useAppStore } from "../store/app";

// POLL_MS é de quanto em quanto tempo a lista é relida enquanto uma captura grava.
const POLL_MS = 1000;

const count = (n: number) => n.toLocaleString("pt-BR");

export function formatBytes(n: number): string {
  if (n < 1024) return `${count(n)} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toLocaleString("pt-BR", { maximumFractionDigits: 1 })} KiB`;
  return `${(n / 1024 / 1024).toLocaleString("pt-BR", { maximumFractionDigits: 1 })} MiB`;
}

const dateTime = (iso: string) =>
  new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "medium" });

// elapsed devolve quantos segundos a captura já gravou, até a duração pedida.
function elapsed(c: Capture, now: number): number {
  const end = c.fim ? new Date(c.fim).getTime() : now;
  return Math.max(0, Math.min(c.duracao_s, Math.round((end - new Date(c.inicio).getTime()) / 1000)));
}

const statusText: Record<Capture["status"], { text: string; color: string }> = {
  gravando: { text: "Gravando", color: "text-destaque-forte" },
  concluida: { text: "Concluída", color: "text-texto" },
  falhou: { text: "Falhou", color: "text-erro" },
};

const message = (e: unknown, fallback: string) => (e instanceof ApiError && e.status !== 500 ? e.message : fallback);

export default function CapturesPage() {
  const sniffer = useAppStore((s) => s.engines.sniffer);
  const targets = useAppStore((s) => s.targets);
  const [captures, setCaptures] = useState<Capture[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [duration, setDuration] = useState("60");
  const [port, setPort] = useState("0");
  const [formError, setFormError] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [now, setNow] = useState(() => Date.now());

  const recording = captures.find((c) => c.status === "gravando");
  const recordingId = recording?.id;

  function reload(): Promise<void> {
    return listCaptures().then(
      (list) => {
        setCaptures(list);
        setError("");
        setLoaded(true);
      },
      () => {
        setError("Não foi possível carregar as capturas.");
        setLoaded(true);
      },
    );
  }

  useEffect(() => {
    void reload();
  }, []);

  // Enquanto uma captura grava, relê a lista e avança o progresso.
  useEffect(() => {
    if (recordingId === undefined) return;
    const t = setInterval(() => {
      setNow(Date.now());
      void reload();
    }, POLL_MS);
    return () => clearInterval(t);
  }, [recordingId]);

  const ports = [...new Set(targets.flatMap(targetPorts))].sort((a, b) => a - b);
  const portOptions = [
    { value: "0", label: "Todas as portas dos alvos" },
    ...ports.map((p) => ({ value: String(p), label: `Porta ${p}` })),
  ];

  async function record(e: FormEvent) {
    e.preventDefault();
    const d = Number(duration);
    if (!Number.isInteger(d) || d < MIN_DURATION || d > MAX_DURATION) {
      setFormError(`A duração precisa ser um número inteiro de ${MIN_DURATION} a ${MAX_DURATION} segundos.`);
      return;
    }
    setFormError("");
    setBusy(true);
    try {
      await startCapture(d, Number(port));
      setNow(Date.now());
    } catch (err) {
      setFormError(message(err, "Não foi possível começar a captura."));
    } finally {
      setBusy(false);
    }
    await reload();
  }

  async function download(c: Capture) {
    try {
      await downloadCapture(c.id);
    } catch (err) {
      setError(message(err, `Não foi possível baixar a captura Nº ${c.id}.`));
    }
  }

  async function remove(c: Capture) {
    if (!window.confirm(`Apagar a captura Nº ${c.id}, de ${dateTime(c.inicio)}? O arquivo .pcap some do servidor.`)) return;
    try {
      await deleteCapture(c.id);
    } catch (err) {
      setError(message(err, `Não foi possível apagar a captura Nº ${c.id}.`));
    }
    await reload();
  }

  return (
    <div className="space-y-8">
      <PageHeader title="Capturas">
        Grava em .pcap, por até {MAX_DURATION / 60} minutos, os pacotes que o sniffer vê nas portas dos alvos. O arquivo
        abre no Wireshark ou na tabela do Inspetor. Cada arquivo tem no máximo 100 MiB, e o diretório inteiro, 1 GiB.
      </PageHeader>

      <Notice>
        Uma captura guarda o tráfego inteiro, com cabeçalhos, cookies e corpos das requisições, e pode conter dados
        pessoais. Grave só o necessário, baixe para um lugar seguro e apague quando terminar. O Omni apaga as capturas
        sozinho depois do prazo de retenção (OMNI_RETENCAO_CAPTURAS, 24 h por padrão).
      </Notice>

      {sniffer !== "ativo" && (
        <Notice>
          {sniffer === "sem-permissao"
            ? "O sniffer está sem permissão de captura, então não dá para gravar. Veja no README como liberar a captura."
            : "O sniffer não está capturando agora. Cadastre um alvo com portas para gravar."}
        </Notice>
      )}

      <form onSubmit={record} aria-label="Nova captura" className="flex flex-wrap items-end gap-3">
        <div className="space-y-1.5">
          <label htmlFor="captura-duracao" className="block text-xs font-medium text-suave">
            Duração (segundos)
          </label>
          <input
            id="captura-duracao"
            inputMode="numeric"
            className="campo w-32 font-mono"
            value={duration}
            onChange={(e) => setDuration(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <span className="block text-xs font-medium text-suave">Porta</span>
          <Select label="Porta" value={port} options={portOptions} onChange={setPort} />
        </div>
        <button type="submit" className="botao" disabled={busy || recording !== undefined || sniffer !== "ativo"}>
          Gravar captura
        </button>
        {formError && (
          <p role="alert" className="w-full text-sm text-erro">
            {formError}
          </p>
        )}
      </form>

      {recording && (
        <section aria-label="Gravação em andamento" className="max-w-xl space-y-2">
          <div className="flex justify-between text-sm">
            <span className="text-texto">
              Gravando a captura Nº {recording.id} · {count(recording.pacotes)} pacotes
            </span>
            <span className="font-mono text-suave tabular-nums">
              {elapsed(recording, now)} s de {recording.duracao_s} s
            </span>
          </div>
          <div
            role="progressbar"
            aria-label="Progresso da gravação"
            aria-valuemin={0}
            aria-valuemax={recording.duracao_s}
            aria-valuenow={elapsed(recording, now)}
            className="h-1.5 overflow-hidden rounded-full bg-elevado"
          >
            <div
              className="h-full rounded-full bg-destaque transition-[width] duration-700"
              style={{ width: `${(elapsed(recording, now) / recording.duracao_s) * 100}%` }}
            />
          </div>
        </section>
      )}

      {error && <p className="text-sm text-erro">{error}</p>}

      {loaded && captures.length === 0 ? (
        <p className="text-sm text-suave">Nenhuma captura gravada ainda.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="tabela">
            <thead>
              <tr>
                {["Nº", "Início", "Duração", "Porta", "Pacotes", "Tamanho", "Status"].map((h) => (
                  <th key={h} scope="col" className="pr-4 whitespace-nowrap">
                    {h}
                  </th>
                ))}
                <th scope="col" className="text-right">
                  Ações
                </th>
              </tr>
            </thead>
            <tbody>
              {captures.map((c) => (
                <tr key={c.id}>
                  <td className="py-2.5 pr-4 font-mono text-apagado tabular-nums">{c.id}</td>
                  <td className="pr-4 whitespace-nowrap">{dateTime(c.inicio)}</td>
                  <td className="pr-4 font-mono tabular-nums">
                    {c.status === "gravando" ? `${c.duracao_s} s` : `${elapsed(c, now)} s`}
                  </td>
                  <td className="pr-4 font-mono">{c.porta === 0 ? "todas" : c.porta}</td>
                  <td className="pr-4 font-mono tabular-nums">
                    {count(c.pacotes)}
                    {c.descartados > 0 && (
                      <span className="block text-xs text-aviso">{count(c.descartados)} descartados</span>
                    )}
                  </td>
                  <td className="pr-4 font-mono whitespace-nowrap tabular-nums">{formatBytes(c.bytes)}</td>
                  <td className={`pr-4 whitespace-nowrap ${statusText[c.status].color}`}>
                    {statusText[c.status].text}
                    {c.erro && <span className="block text-xs text-suave">{c.erro}</span>}
                  </td>
                  <td>
                    {c.status !== "gravando" && (
                      <div className="flex justify-end gap-1 whitespace-nowrap">
                        <button
                          type="button"
                          aria-label={`Baixar a captura ${c.id}`}
                          onClick={() => void download(c)}
                          className="botao-leve px-2 py-1"
                        >
                          Baixar
                        </button>
                        <Link
                          to={`/inspetor/captura/${c.id}`}
                          aria-label={`Abrir a captura ${c.id} no Inspetor`}
                          className="botao-leve px-2 py-1"
                        >
                          Abrir no Inspetor
                        </Link>
                        <button
                          type="button"
                          aria-label={`Apagar a captura ${c.id}`}
                          onClick={() => void remove(c)}
                          className="botao-leve px-2 py-1 text-erro hover:bg-erro/10 hover:text-erro"
                        >
                          Apagar
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
