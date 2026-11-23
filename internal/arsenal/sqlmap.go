package arsenal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// SqlmapImage é a imagem do SQLMap, com versão fixa: o projeto não publica
// imagem oficial, então é a googlesky/sqlmap, fixada pelo commit do upstream.
const SqlmapImage = "googlesky/sqlmap:upstream-5db2e62884d91f3ed3b43ec55904737e4051e9d7"

var (
	// sqlmapRoute é o que uma rota pode conter: caminho absoluto com letras,
	// dígitos e / _ . : -. Os "..", "//" e o que sobrar são vetados à parte.
	sqlmapRoute = regexp.MustCompile(`^/[A-Za-z0-9/_.:-]*$`)
	// sqlmapParam é o nome do parâmetro a testar.
	sqlmapParam = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	// sqlmapFile é um caminho absoluto no servidor alvo, para a ação de ler
	// um arquivo.
	sqlmapFile = regexp.MustCompile(`^/[A-Za-z0-9/_.-]{1,255}$`)
)

// sqlmapKeys são as opções aceitas; qualquer outra chave é recusada.
var sqlmapKeys = map[string]bool{
	"rota": true, "parametro": true, "porta": true, "nivel": true, "risco": true,
	"tecnica": true, "sgbd": true, "threads": true, "tamper": true, "acao": true, "arquivo": true,
}

// Conjuntos fechados das opções do SQLMap. A interface manda rótulos em
// linguagem comum; aqui cada valor vira a flag correspondente.

// sqlmapTechnique é a técnica de injeção; "" deixa o SQLMap usar todas.
var sqlmapTechnique = map[string]bool{"": true, "B": true, "E": true, "U": true, "S": true, "T": true, "Q": true}

// sqlmapDBMS restringe a varredura a um banco; "" deixa o SQLMap detectar.
var sqlmapDBMS = map[string]bool{
	"": true, "MySQL": true, "PostgreSQL": true, "MariaDB": true, "Microsoft SQL Server": true,
	"Oracle": true, "SQLite": true, "IBM DB2": true, "Firebird": true, "Sybase": true,
	"Microsoft Access": true, "SAP MaxDB": true,
}

// sqlmapTamper é o script de evasão de WAF; "" não usa nenhum.
var sqlmapTamper = map[string]bool{
	"": true, "space2comment": true, "between": true, "charencode": true, "randomcase": true,
	"apostrophemask": true, "base64encode": true, "equaltolike": true, "percentage": true,
}

// sqlmapAction é o que fazer depois de detectar a injeção, da só-detecção à
// extração de dados e ao shell. Cada ação vira as flags do SQLMap; a ação
// "ler-arquivo" usa à parte o caminho da opção "arquivo".
var sqlmapAction = map[string][]string{
	"detectar":    {},
	"fingerprint": {"--fingerprint"},
	"banner":      {"--banner", "--current-user", "--current-db", "--is-dba", "--hostname"},
	"bancos":      {"--dbs"},
	"tabelas":     {"--tables"},
	"colunas":     {"--columns"},
	"dump":        {"--dump"},
	"dump-all":    {"--dump-all"},
	"sql-shell":   {"--sql-shell"},
	"os-shell":    {"--os-shell"},
	"ler-arquivo": {},
}

// sqlmapConfig é o conjunto validado das opções, pronto para virar argumentos.
type sqlmapConfig struct {
	route, param         string
	port                 int
	level, risk, threads int
	technique, dbms      string
	tamper, action, file string
}

// sqlmapNumber lê uma opção numérica dentro de uma faixa fechada; ausente usa
// o padrão def.
func sqlmapNumber(o Options, key, def string, min, max int) (int, bool) {
	v := o[key]
	if v == "" {
		v = def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min || n > max {
		return 0, false
	}
	return n, true
}

// sqlmapOptions valida todas as opções do SQLMap. A rota vem normalizada pelo
// sniffer (":id" no lugar dos identificadores) e cada ":id" vira 1 na URL
// testada.
func sqlmapOptions(t target.Target, o Options) (sqlmapConfig, error) {
	var c sqlmapConfig
	bad := func(key string) (sqlmapConfig, error) { return sqlmapConfig{}, &OptionError{Tool: "sqlmap", Key: key} }
	for k := range o {
		if !sqlmapKeys[k] {
			return bad(k)
		}
	}
	c.route = o["rota"]
	if !sqlmapRoute.MatchString(c.route) || strings.Contains(c.route, "..") || strings.Contains(c.route, "//") {
		return bad("rota")
	}
	c.param = o["parametro"]
	if !sqlmapParam.MatchString(c.param) {
		return bad("parametro")
	}
	port, err := appPort("sqlmap", t, o)
	if err != nil {
		return sqlmapConfig{}, err
	}
	c.port = port

	var ok bool
	if c.level, ok = sqlmapNumber(o, "nivel", "1", 1, 5); !ok {
		return bad("nivel")
	}
	if c.risk, ok = sqlmapNumber(o, "risco", "1", 1, 3); !ok {
		return bad("risco")
	}
	if c.threads, ok = sqlmapNumber(o, "threads", "1", 1, 10); !ok {
		return bad("threads")
	}
	c.technique = o["tecnica"]
	if !sqlmapTechnique[c.technique] {
		return bad("tecnica")
	}
	c.dbms = o["sgbd"]
	if !sqlmapDBMS[c.dbms] {
		return bad("sgbd")
	}
	c.tamper = o["tamper"]
	if !sqlmapTamper[c.tamper] {
		return bad("tamper")
	}
	c.action = o["acao"]
	if c.action == "" {
		c.action = "detectar"
	}
	if _, known := sqlmapAction[c.action]; !known {
		return bad("acao")
	}
	c.file = o["arquivo"]
	if c.action == "ler-arquivo" {
		if !sqlmapFile.MatchString(c.file) {
			return bad("arquivo")
		}
	} else if c.file != "" {
		return bad("arquivo")
	}

	segs := strings.Split(c.route, "/")
	for i, s := range segs {
		if s == ":id" {
			segs[i] = "1"
		}
	}
	c.route = strings.Join(segs, "/")
	return c, nil
}

// SqlmapArgs devolve os argumentos do SQLMap para testar um parâmetro GET de
// uma rota. Sem opções, roda só a detecção (nível 1, risco 1, uma thread,
// sessão sempre nova e nada de extração, shell ou arquivos). As opções da
// interface sobem nível e risco, escolhem técnica, banco, threads e evasão, e
// a ação pode escalar para enumerar, extrair dados, abrir shell ou ler um
// arquivo do servidor.
func SqlmapArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	c, err := sqlmapOptions(t, o)
	if err != nil {
		return nil, err
	}
	u := "http://" + net.JoinHostPort(t.Host, strconv.Itoa(c.port)) + c.route + "?" + c.param + "=1"
	args := []string{"-u", u, "-p", c.param, "--batch",
		"--level", strconv.Itoa(c.level), "--risk", strconv.Itoa(c.risk),
		"--threads", strconv.Itoa(c.threads), "--flush-session", "--disable-coloring", "--user-agent", UserAgent}
	if c.technique != "" {
		args = append(args, "--technique", c.technique)
	}
	if c.dbms != "" {
		args = append(args, "--dbms", c.dbms)
	}
	if c.tamper != "" {
		args = append(args, "--tamper", c.tamper)
	}
	args = append(args, sqlmapAction[c.action]...)
	if c.action == "ler-arquivo" {
		args = append(args, "--file-read="+c.file)
	}
	return args, nil
}

// SqlmapReport é o veredito do SQLMap para o parâmetro testado.
type SqlmapReport struct {
	URL        string            `json:"url"`
	Parameter  string            `json:"parametro"`
	Injectable bool              `json:"injetavel"`
	DBMS       string            `json:"sgbd"`
	Techniques []SqlmapTechnique `json:"tecnicas"`
}

type SqlmapTechnique struct {
	Type    string `json:"tipo"`
	Title   string `json:"titulo"`
	Payload string `json:"payload"`
}

var (
	// sqlmapURL é a linha "[INFO] testing URL 'http://...'".
	sqlmapURL = regexp.MustCompile(`testing URL '([^']+)'`)
	// sqlmapParamLine abre o bloco de uma injeção: "Parameter: id (GET)".
	sqlmapParamLine = regexp.MustCompile(`^Parameter: ([A-Za-z_][A-Za-z0-9_]*) \(`)
	// sqlmapTested é "testing if GET parameter 'id' is dynamic", que dá o
	// parâmetro quando nada foi encontrado.
	sqlmapTested = regexp.MustCompile(`testing if \S+ parameter '([^']+)'`)
)

// sqlmapParse lê o texto da saída padrão do SQLMap: os blocos
// "Parameter / Type / Title / Payload" e a linha "back-end DBMS".
func sqlmapParse(stdout []byte) SqlmapReport {
	rep := SqlmapReport{Techniques: []SqlmapTechnique{}}
	var cur *SqlmapTechnique
	flush := func() {
		if cur != nil {
			rep.Techniques = append(rep.Techniques, *cur)
			cur = nil
		}
	}
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := sqlmapURL.FindStringSubmatch(line); m != nil && rep.URL == "" {
			rep.URL = m[1]
		}
		if m := sqlmapTested.FindStringSubmatch(line); m != nil && rep.Parameter == "" {
			rep.Parameter = m[1]
		}
		switch {
		case sqlmapParamLine.MatchString(line):
			flush()
			rep.Parameter = sqlmapParamLine.FindStringSubmatch(line)[1]
		case strings.HasPrefix(line, "Type: "):
			flush()
			cur = &SqlmapTechnique{Type: strings.TrimPrefix(line, "Type: ")}
		case strings.HasPrefix(line, "Title: ") && cur != nil:
			cur.Title = strings.TrimPrefix(line, "Title: ")
		case strings.HasPrefix(line, "Payload: ") && cur != nil:
			cur.Payload = strings.TrimPrefix(line, "Payload: ")
			flush()
		case strings.HasPrefix(line, "back-end DBMS: "):
			rep.DBMS = strings.TrimPrefix(line, "back-end DBMS: ")
		}
	}
	flush()
	rep.Injectable = len(rep.Techniques) > 0
	return rep
}

// ParseSqlmap devolve um achado crítico por técnica de injeção encontrada;
// parâmetro não injetável não gera nenhum.
func ParseSqlmap(r io.Reader) ([]findings.Finding, error) {
	stdout, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	rep := sqlmapParse(stdout)
	out := []findings.Finding{}
	for _, t := range rep.Techniques {
		title := fmt.Sprintf("Injeção de SQL no parâmetro %s: %s", rep.Parameter, t.Type)
		evidence := t.Title
		if rep.DBMS != "" {
			evidence += " · " + rep.DBMS
		}
		out = append(out, findings.New("sqlmap", truncate(title, maxEvidence), truncate(evidence, maxEvidence), findings.Critical, 0))
	}
	return out, nil
}

// SqlmapReportFrom monta o SqlmapReport a partir do texto do SQLMap.
func SqlmapReportFrom(stdout []byte) (json.RawMessage, error) {
	return json.Marshal(sqlmapParse(stdout))
}
