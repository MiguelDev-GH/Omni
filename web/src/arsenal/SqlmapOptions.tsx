import { type ReactNode, useEffect, useState } from "react";
import { listRoutes } from "../api/requests";
import type { Target } from "../api/targets";
import { Notice } from "../components/Card";
import Select, { type Option } from "../components/Select";
import { PortField } from "./common";

// parameter é o padrão que o backend aceita para o nome do parâmetro.
const parameter = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/;
// filePath é um caminho absoluto no servidor alvo, para a ação de ler arquivo.
const filePath = /^\/[A-Za-z0-9/_.-]{1,255}$/;

// As opções do SQLMap em linguagem comum: o valor é o que o backend espera, o
// rótulo e a descrição explicam o que a escolha faz, sem citar a flag.
const niveis: Option<string>[] = [
  { value: "1", label: "Nível 1 — padrão", description: "Testa só o parâmetro escolhido. Mais rápido e discreto." },
  { value: "2", label: "Nível 2", description: "Inclui também testes no cabeçalho Cookie." },
  { value: "3", label: "Nível 3", description: "Inclui os cabeçalhos User-Agent e Referer." },
  { value: "4", label: "Nível 4", description: "Mais vetores de teste por requisição." },
  { value: "5", label: "Nível 5 — exaustivo", description: "Todos os vetores e cabeçalhos. Mais lento e barulhento." },
];

const riscos: Option<string>[] = [
  { value: "1", label: "Risco 1 — padrão", description: "Só payloads inofensivos para a aplicação." },
  { value: "2", label: "Risco 2", description: "Acrescenta testes por tempo, com consultas pesadas." },
  { value: "3", label: "Risco 3 — agressivo", description: "Inclui OR e UPDATE, que podem alterar dados. Cuidado." },
];

const tecnicas: Option<string>[] = [
  { value: "", label: "Todas as técnicas", description: "Deixa o SQLMap tentar todas as formas de injeção." },
  { value: "B", label: "Booleana cega", description: "Infere pela diferença entre respostas verdadeiro e falso." },
  { value: "E", label: "Baseada em erro", description: "Extrai dados das mensagens de erro do banco." },
  { value: "U", label: "União (UNION)", description: "Junta o resultado de outra consulta à da página." },
  { value: "S", label: "Consultas empilhadas", description: "Executa comandos extras separados por ponto e vírgula." },
  { value: "T", label: "Cega por tempo", description: "Infere pelo atraso proposital nas respostas." },
  { value: "Q", label: "Consulta embutida", description: "Injeta uma subconsulta dentro da original." },
];

const bancos: Option<string>[] = [
  { value: "", label: "Detectar sozinho", description: "O SQLMap descobre o banco pela resposta." },
  { value: "PostgreSQL", label: "PostgreSQL", description: "Pula a detecção e assume PostgreSQL." },
  { value: "MySQL", label: "MySQL", description: "Pula a detecção e assume MySQL." },
  { value: "MariaDB", label: "MariaDB", description: "Pula a detecção e assume MariaDB." },
  { value: "Microsoft SQL Server", label: "SQL Server", description: "Pula a detecção e assume Microsoft SQL Server." },
  { value: "Oracle", label: "Oracle", description: "Pula a detecção e assume Oracle." },
  { value: "SQLite", label: "SQLite", description: "Pula a detecção e assume SQLite." },
];

const quantasThreads: Option<string>[] = [
  { value: "1", label: "1 thread — padrão", description: "Uma requisição por vez. Mais estável." },
  { value: "2", label: "2 threads", description: "Dobra o paralelismo." },
  { value: "5", label: "5 threads", description: "Mais rápido, com mais carga no alvo." },
  { value: "10", label: "10 threads — máximo", description: "O mais rápido e o mais pesado para o alvo." },
];

const evasoes: Option<string>[] = [
  { value: "", label: "Sem evasão", description: "Envia os payloads como estão." },
  { value: "space2comment", label: "Espaços viram comentários", description: "Troca espaços por /**/ para driblar filtros." },
  { value: "between", label: "Troca '>' por BETWEEN", description: "Evita o sinal de maior, muitas vezes bloqueado." },
  { value: "charencode", label: "Codifica em URL", description: "Aplica percent-encoding aos caracteres do payload." },
  { value: "randomcase", label: "Caixa aleatória", description: "Mistura maiúsculas e minúsculas nas palavras-chave." },
  { value: "apostrophemask", label: "Mascara aspas", description: "Troca a aspa simples por seu equivalente UTF-8." },
  { value: "equaltolike", label: "Troca '=' por LIKE", description: "Evita o sinal de igual nas comparações." },
  { value: "base64encode", label: "Codifica em Base64", description: "Embrulha o payload em Base64." },
  { value: "percentage", label: "Insere '%'", description: "Põe % antes de cada caractere, no estilo ASP." },
];

const acoes: Option<string>[] = [
  { value: "detectar", label: "Só detectar", description: "Confirma se há injeção e para. Não lê dados." },
  { value: "fingerprint", label: "Identificar o banco", description: "Descobre a versão do banco e o sistema operacional." },
  { value: "banner", label: "Banner e usuário", description: "Lê banner, usuário e banco atuais e se é DBA." },
  { value: "bancos", label: "Listar os bancos", description: "Enumera os bancos de dados disponíveis." },
  { value: "tabelas", label: "Listar as tabelas", description: "Enumera as tabelas do banco atual." },
  { value: "colunas", label: "Listar as colunas", description: "Enumera as colunas das tabelas." },
  { value: "dump", label: "Extrair dados", description: "Baixa o conteúdo das tabelas encontradas. Ofensivo." },
  { value: "dump-all", label: "Extrair tudo", description: "Baixa todos os bancos inteiros. Muito ofensivo e lento." },
  { value: "sql-shell", label: "Shell SQL", description: "Abre um prompt para rodar SQL direto no banco." },
  { value: "os-shell", label: "Shell no sistema", description: "Tenta executar comandos no servidor. Muito ofensivo." },
  { value: "ler-arquivo", label: "Ler um arquivo", description: "Lê um arquivo do servidor pelo caminho informado." },
];

// Field alinha um rótulo ao seu controle, como o resto do formulário.
function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <label htmlFor={id} className="text-suave">
        {label}
      </label>
      {children}
    </div>
  );
}

export default function SqlmapOptions({
  value,
  onChange,
  target,
}: {
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  target: Target;
}) {
  // routes fica null enquanto carrega e vazio quando o sniffer não viu
  // nenhuma rota GET (ou a busca falhou): sem rota vista, não há o que testar.
  const [routes, setRoutes] = useState<string[] | null>(null);
  const [text, setText] = useState(value.parametro ?? "");
  const [file, setFile] = useState(value.arquivo ?? "");

  useEffect(() => {
    let live = true;
    listRoutes().then(
      (list) => live && setRoutes([...new Set(list.filter((r) => r.metodo === "GET").map((r) => r.rota))].sort()),
      () => live && setRoutes([]),
    );
    return () => {
      live = false;
    };
  }, []);

  if (routes === null) return <p className="text-sm text-suave">Carregando as rotas vistas no tráfego...</p>;
  if (routes.length === 0)
    return (
      <Notice tone="info">
        O SQLMap precisa de tráfego real na aplicação: nenhuma rota GET foi vista nas últimas 24 horas. Use a aplicação com o sniffer ativo e volte aqui.
      </Notice>
    );

  // set escreve uma opção do select, que já vem de uma lista fechada.
  const set = (key: string) => (v: string) => onChange({ ...value, [key]: v });

  const invalid = text !== "" && !parameter.test(text);
  // Só um parâmetro válido segue para a varredura: um nome inválido some das
  // opções e deixa o botão desabilitado.
  const setParam = (v: string) => {
    setText(v);
    const next: Record<string, string | undefined> = { ...value, parametro: v };
    if (!parameter.test(v)) delete next.parametro;
    onChange(next as Record<string, string>);
  };

  const action = value.acao ?? "detectar";
  // Ao sair da ação de ler arquivo, o caminho não acompanha a varredura.
  const setAction = (acao: string) => {
    const next: Record<string, string | undefined> = { ...value, acao };
    if (acao !== "ler-arquivo") delete next.arquivo;
    onChange(next as Record<string, string>);
  };

  const fileInvalid = file !== "" && !filePath.test(file);
  // O caminho do arquivo segue a mesma regra do parâmetro: inválido não vai.
  const setFilePath = (v: string) => {
    setFile(v);
    const next: Record<string, string | undefined> = { ...value, arquivo: v };
    if (!filePath.test(v)) delete next.arquivo;
    onChange(next as Record<string, string>);
  };

  return (
    <div className="flex flex-col gap-4 text-sm">
      <div className="flex flex-wrap items-end gap-x-6 gap-y-3">
        <Field id="sqlmap-rota" label="Rota">
          <Select
            id="sqlmap-rota"
            value={value.rota ?? null}
            onChange={set("rota")}
            options={routes.map((r) => ({ value: r, label: r }))}
            placeholder="Escolha uma rota"
          />
        </Field>
        <Field id="sqlmap-parametro" label="Parâmetro">
          <input
            id="sqlmap-parametro"
            value={text}
            onChange={(e) => setParam(e.target.value)}
            placeholder="ex.: q"
            maxLength={64}
            autoComplete="off"
            spellCheck={false}
            aria-invalid={invalid}
            className="campo font-mono"
          />
        </Field>
        <PortField id="sqlmap-porta" value={value} onChange={onChange} target={target} />
      </div>

      <div className="flex flex-wrap items-end gap-x-6 gap-y-3">
        <Field id="sqlmap-nivel" label="Intensidade">
          <Select id="sqlmap-nivel" value={value.nivel ?? "1"} onChange={set("nivel")} options={niveis} />
        </Field>
        <Field id="sqlmap-risco" label="Risco">
          <Select id="sqlmap-risco" value={value.risco ?? "1"} onChange={set("risco")} options={riscos} />
        </Field>
        <Field id="sqlmap-tecnica" label="Técnica">
          <Select id="sqlmap-tecnica" value={value.tecnica ?? ""} onChange={set("tecnica")} options={tecnicas} />
        </Field>
        <Field id="sqlmap-sgbd" label="Banco-alvo">
          <Select id="sqlmap-sgbd" value={value.sgbd ?? ""} onChange={set("sgbd")} options={bancos} />
        </Field>
        <Field id="sqlmap-threads" label="Threads">
          <Select id="sqlmap-threads" value={value.threads ?? "1"} onChange={set("threads")} options={quantasThreads} />
        </Field>
        <Field id="sqlmap-tamper" label="Evasão de WAF">
          <Select id="sqlmap-tamper" value={value.tamper ?? ""} onChange={set("tamper")} options={evasoes} />
        </Field>
      </div>

      <div className="flex flex-wrap items-end gap-x-6 gap-y-3">
        <Field id="sqlmap-acao" label="Ação">
          <Select id="sqlmap-acao" value={action} onChange={setAction} options={acoes} className="min-w-52" />
        </Field>
        {action === "ler-arquivo" && (
          <Field id="sqlmap-arquivo" label="Arquivo">
            <input
              id="sqlmap-arquivo"
              value={file}
              onChange={(e) => setFilePath(e.target.value)}
              placeholder="ex.: /etc/passwd"
              maxLength={256}
              autoComplete="off"
              spellCheck={false}
              aria-invalid={fileInvalid}
              className="campo font-mono"
            />
          </Field>
        )}
      </div>
    </div>
  );
}
