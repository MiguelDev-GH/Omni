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
)

// sqlmapOptions valida as opções do SQLMap: rota, parâmetro e porta. A rota
// vem normalizada pelo sniffer (":id" no lugar dos identificadores) e cada
// ":id" vira 1 na URL testada.
func sqlmapOptions(t target.Target, o Options) (route, param string, port int, err error) {
	for k := range o {
		if k != "rota" && k != "parametro" && k != "porta" {
			return "", "", 0, &OptionError{Tool: "sqlmap", Key: k}
		}
	}
	route = o["rota"]
	if !sqlmapRoute.MatchString(route) || strings.Contains(route, "..") || strings.Contains(route, "//") {
		return "", "", 0, &OptionError{Tool: "sqlmap", Key: "rota"}
	}
	param = o["parametro"]
	if !sqlmapParam.MatchString(param) {
		return "", "", 0, &OptionError{Tool: "sqlmap", Key: "parametro"}
	}
	port, err = appPort("sqlmap", t, o)
	if err != nil {
		return "", "", 0, err
	}
	segs := strings.Split(route, "/")
	for i, s := range segs {
		if s == ":id" {
			segs[i] = "1"
		}
	}
	return strings.Join(segs, "/"), param, port, nil
}

// SqlmapArgs devolve os argumentos do SQLMap para testar um parâmetro GET de
// uma rota: só nível 1 e risco 1, uma thread, sessão sempre nova e nenhuma
// opção de extração de dados, de shell ou de arquivos.
func SqlmapArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	route, param, port, err := sqlmapOptions(t, o)
	if err != nil {
		return nil, err
	}
	u := "http://" + net.JoinHostPort(t.Host, strconv.Itoa(port)) + route + "?" + param + "=1"
	return []string{"-u", u, "-p", param, "--batch", "--level", "1", "--risk", "1", "--threads", "1",
		"--flush-session", "--disable-coloring", "--user-agent", UserAgent}, nil
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
