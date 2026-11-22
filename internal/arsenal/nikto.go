package arsenal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

const (
	// NiktoImage é a imagem oficial do Nikto, com versão fixa.
	NiktoImage = "ghcr.io/sullo/nikto:2.6.1"
	// niktoOutput é onde o Nikto grava o JSON no container: ele só aceita
	// nome terminado em .json e não escreve o JSON na saída padrão.
	niktoOutput = "/tmp/nikto.json"
)

// NiktoArgs devolve os argumentos do Nikto para uma porta de aplicação, com
// o JSON em niktoOutput. -ask no desliga a pergunta sobre enviar banners ao
// CIRT.net, que -nointeractive não desliga; -maxtime encerra o Nikto antes
// dos 10 minutos da varredura, ainda gravando o JSON.
func NiktoArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	for k := range o {
		if k != "porta" {
			return nil, &OptionError{Tool: "nikto", Key: k}
		}
	}
	port, err := appPort("nikto", t, o)
	if err != nil {
		return nil, err
	}
	return []string{"-h", "http://" + net.JoinHostPort(t.Host, strconv.Itoa(port)), "-Format", "json", "-o", niktoOutput,
		"-nointeractive", "-ask", "no", "-useragent", UserAgent, "-maxtime", "540s"}, nil
}

// NiktoReport é o servidor identificado e os itens que o Nikto relatou.
type NiktoReport struct {
	Server string      `json:"servidor"`
	Items  []NiktoItem `json:"itens"`
}

type NiktoItem struct {
	ID         string   `json:"id"`
	Method     string   `json:"metodo"`
	URL        string   `json:"url"`
	Message    string   `json:"mensagem"`
	References []string `json:"referencias"`
	// Category é cabecalhos, arquivos, versao ou outros (niktoCategory).
	Category string `json:"categoria"`
}

type niktoHost struct {
	Port  json.Number `json:"port"`
	Vulns []struct {
		ID         string `json:"id"`
		Method     string `json:"method"`
		Msg        string `json:"msg"`
		References string `json:"references"`
		URL        string `json:"url"`
	} `json:"vulnerabilities"`
}

var (
	// niktoLine é um item no console: "+ [007226] /.env: .env file found.".
	niktoLine = regexp.MustCompile(`^\+ \[(\d+)\] (\S*): (.+)$`)
	// niktoSee é o "See: <url>" que o console acrescenta às mensagens.
	niktoSee = regexp.MustCompile(` See: \S+$`)
)

// niktoOutputParts separa o console do arquivo JSON; file é nil quando o
// arquivo não veio (o Nikto morreu antes de gravá-lo).
func niktoOutputParts(out []byte) (console, file []byte) {
	console, file, found := bytes.Cut(out, []byte(OutputFileMarker))
	if !found {
		return out, nil
	}
	return console, file
}

// niktoServer devolve o banner da linha "+ Server: ..." do console: o JSON
// do Nikto 2.6.1 o grava antes de conhecê-lo, sempre como null.
func niktoServer(console []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(console))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "+ Server: "); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func niktoJSON(file []byte) ([]niktoHost, error) {
	if file == nil {
		return nil, errors.New("saída do nikto incompleta")
	}
	var hosts []niktoHost
	if err := json.Unmarshal(file, &hosts); err != nil {
		return nil, fmt.Errorf("ler JSON do nikto: %w", err)
	}
	return hosts, nil
}

// ParseNiktoJSON devolve uma Finding por item do Nikto, com severidade
// derivada da categoria. Sem o JSON (ou com ele cortado), lê os itens do
// console, sem porta.
func ParseNiktoJSON(r io.Reader) ([]findings.Finding, error) {
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	console, file := niktoOutputParts(out)
	list := []findings.Finding{}
	if hosts, err := niktoJSON(file); err == nil {
		for _, h := range hosts {
			port, _ := strconv.Atoi(h.Port.String())
			for _, v := range h.Vulns {
				list = append(list, niktoFinding(v.URL, v.Msg, port))
			}
		}
		return list, nil
	}
	sc := bufio.NewScanner(bytes.NewReader(console))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		m := niktoLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		list = append(list, niktoFinding(m[2], niktoSee.ReplaceAllString(m[3], ""), 0))
	}
	return list, sc.Err()
}

func niktoFinding(url, msg string, port int) findings.Finding {
	sev := findings.Info
	switch niktoCategory(msg) {
	case "cabecalhos":
		sev = findings.Low
	case "arquivos", "versao":
		sev = findings.Medium
	}
	return findings.New("nikto", truncate(url+": "+msg, maxEvidence), "", sev, port)
}

// niktoCategory classifica a mensagem de um item, nesta ordem:
//   - versao: software desatualizado ("outdated", "end of life");
//   - cabecalhos: cabeçalhos HTTP e cookies ("header", "cookie");
//   - arquivos: arquivos e diretórios expostos ("file found", "directory
//     indexing", "backup", "directory found");
//   - outros: o resto.
func niktoCategory(msg string) string {
	m := strings.ToLower(msg)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(m, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("outdated", "end of life"):
		return "versao"
	case has("header", "cookie"):
		return "cabecalhos"
	case has("file found", "directory indexing", "backup", "directory found"):
		return "arquivos"
	}
	return "outros"
}

// NiktoReportFrom monta o NiktoReport a partir do console e do JSON do
// Nikto; sem o JSON completo, recusa.
func NiktoReportFrom(out []byte) (json.RawMessage, error) {
	console, file := niktoOutputParts(out)
	hosts, err := niktoJSON(file)
	if err != nil {
		return nil, err
	}
	rep := NiktoReport{Server: niktoServer(console), Items: []NiktoItem{}}
	for _, h := range hosts {
		for _, v := range h.Vulns {
			rep.Items = append(rep.Items, NiktoItem{
				ID: v.ID, Method: v.Method, URL: v.URL, Message: v.Msg,
				References: nonNil(strings.FieldsFunc(v.References, func(r rune) bool { return r == ',' || r == ' ' })),
				Category:   niktoCategory(v.Msg),
			})
		}
	}
	return json.Marshal(rep)
}
