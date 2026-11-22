package arsenal

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

const (
	// FfufImage é a imagem do ffuf, com versão fixa (Alpine, mantida pela
	// SecSI: o projeto não publica imagem oficial).
	FfufImage = "secsi/ffuf:2.3.0"
	// ffufWordlist é onde a lista de palavras é copiada no container.
	ffufWordlist = "/omni/lista.txt"
)

// As listas são do SecLists (licença MIT em wordlists/LICENSE).
//
//go:embed wordlists/common.txt wordlists/raft-small-words.txt
var wordlists embed.FS

// ffufLists são as listas aceitas na opção "lista", com o arquivo de cada uma.
var ffufLists = map[string]string{
	"pequena": "wordlists/common.txt",           // Discovery/Web-Content/common.txt
	"media":   "wordlists/raft-small-words.txt", // Discovery/Web-Content/raft-small-words.txt
}

// ffufOptions valida as opções do ffuf e devolve o arquivo da lista e a porta.
func ffufOptions(t target.Target, o Options) (list string, port int, err error) {
	for k := range o {
		if k != "lista" && k != "porta" {
			return "", 0, &OptionError{Tool: "ffuf", Key: k}
		}
	}
	list, err = ffufList(o)
	if err != nil {
		return "", 0, err
	}
	port = t.AppPorts[0]
	if v, ok := o["porta"]; ok {
		p, err := strconv.Atoi(v)
		if err != nil || !slices.Contains(t.AppPorts, p) {
			return "", 0, &OptionError{Tool: "ffuf", Key: "porta"}
		}
		port = p
	}
	return list, port, nil
}

func ffufList(o Options) (string, error) {
	name := "pequena"
	if v, ok := o["lista"]; ok {
		name = v
	}
	file, ok := ffufLists[name]
	if !ok {
		return "", &OptionError{Tool: "ffuf", Key: "lista"}
	}
	return file, nil
}

// FfufArgs devolve os argumentos do ffuf: fuzzing do primeiro segmento do
// caminho numa porta de aplicação, com o JSON na saída padrão. Aceita todos
// os status menos 404, e a autocalibração (-ac) descarta as respostas que
// imitam o "não encontrado", como as de aplicações de página única. Com 100
// requisições por segundo, a lista média (43 mil palavras) cabe nos 10
// minutos da varredura; -maxtime encerra o ffuf antes, gravando o JSON.
func FfufArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	_, port, err := ffufOptions(t, o)
	if err != nil {
		return nil, err
	}
	return []string{"-u", "http://" + net.JoinHostPort(t.Host, strconv.Itoa(port)) + "/FUZZ", "-w", ffufWordlist,
		"-of", "json", "-o", "/dev/stdout", "-s", "-t", "10", "-rate", "100", "-maxtime", "540",
		"-mc", "all", "-fc", "404", "-ac", "-H", "User-Agent: " + UserAgent}, nil
}

// FfufFiles devolve a lista de palavras escolhida, copiada para o container.
func FfufFiles(o Options) (map[string][]byte, error) {
	file, err := ffufList(o)
	if err != nil {
		return nil, err
	}
	b, err := wordlists.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{ffufWordlist: b}, nil
}

type ffufOutput struct {
	Results []struct {
		Input struct {
			FUZZ string `json:"FUZZ"`
		} `json:"input"`
		Status   int    `json:"status"`
		Length   int    `json:"length"`
		Words    int    `json:"words"`
		Redirect string `json:"redirectlocation"`
		Duration int64  `json:"duration"` // nanossegundos
		URL      string `json:"url"`
		Host     string `json:"host"`
	} `json:"results"`
	// Da configuração só a URL é lida: o resto, como os cabeçalhos, não
	// interessa ao relatório.
	Config struct {
		URL string `json:"url"`
	} `json:"config"`
}

// ffufJSON acha o JSON que o ffuf grava no fim da saída padrão, depois das
// palavras que o modo -s escreve a cada resultado. found diz se ele existe.
func ffufJSON(stdout []byte) (out ffufOutput, found bool, err error) {
	i := bytes.Index(stdout, []byte(`{"commandline"`))
	if i < 0 {
		return out, false, nil
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout[i:]), &out); err != nil {
		return out, true, fmt.Errorf("ler JSON do ffuf: %w", err)
	}
	return out, true, nil
}

// ParseFfufJSON devolve uma Finding por caminho encontrado, info ou média
// para caminhos sensíveis. Sem o JSON final (saída cortada), usa as palavras
// do modo -s, sem status nem porta.
func ParseFfufJSON(r io.Reader) ([]findings.Finding, error) {
	stdout, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	out := []findings.Finding{}
	res, found, err := ffufJSON(stdout)
	if found && err == nil {
		for _, x := range res.Results {
			path := "/" + x.Input.FUZZ
			evidence := fmt.Sprintf("HTTP %d · %d bytes · %s", x.Status, x.Length, x.URL)
			if x.Redirect != "" {
				evidence += " → " + x.Redirect
			}
			_, p, _ := net.SplitHostPort(x.Host)
			port, _ := strconv.Atoi(p)
			out = append(out, ffufFinding(path, truncate(evidence, maxEvidence), port))
		}
		return out, nil
	}
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	for sc.Scan() {
		word := strings.TrimSpace(sc.Text())
		if word == "" || strings.HasPrefix(word, "{") {
			continue
		}
		out = append(out, ffufFinding("/"+word, "", 0))
	}
	return out, sc.Err()
}

func ffufFinding(path, evidence string, port int) findings.Finding {
	sev := findings.Info
	if sensitivePath(path) {
		sev = findings.Medium
	}
	return findings.New("ffuf", "Caminho encontrado: "+truncate(path, maxEvidence), evidence, sev, port)
}

// sensitivePath diz se o caminho costuma expor código, configuração ou
// administração: um segmento .git ou .env, um nome com "backup", "admin" ou
// "phpinfo", server-status, ou um arquivo terminado em .bak ou .sql.
func sensitivePath(path string) bool {
	for _, seg := range strings.Split(strings.ToLower(path), "/") {
		switch {
		case seg == ".git", seg == ".env", strings.HasPrefix(seg, ".env."), seg == "server-status",
			strings.Contains(seg, "backup"), strings.HasPrefix(seg, "admin"), strings.HasPrefix(seg, "phpinfo"),
			strings.HasSuffix(seg, ".bak"), strings.HasSuffix(seg, ".sql"):
			return true
		}
	}
	return false
}

// FfufReport são os caminhos encontrados, em ordem alfabética.
type FfufReport struct {
	BaseURL string       `json:"base_url"`
	Results []FfufResult `json:"resultados"`
}

type FfufResult struct {
	Path       string `json:"caminho"`
	Status     int    `json:"status"`
	Size       int    `json:"tamanho"`
	Words      int    `json:"palavras"`
	Redirect   string `json:"redirecionamento"`
	DurationMS int    `json:"duracao_ms"`
}

// FfufReportFrom monta o FfufReport a partir do JSON do ffuf; a saída sem
// ele foi cortada e é recusada.
func FfufReportFrom(stdout []byte) (json.RawMessage, error) {
	res, found, err := ffufJSON(stdout)
	if !found {
		return nil, errors.New("saída do ffuf incompleta")
	}
	if err != nil {
		return nil, err
	}
	rep := FfufReport{BaseURL: strings.TrimSuffix(res.Config.URL, "/FUZZ"), Results: []FfufResult{}}
	for _, x := range res.Results {
		rep.Results = append(rep.Results, FfufResult{
			Path: "/" + x.Input.FUZZ, Status: x.Status, Size: x.Length, Words: x.Words,
			Redirect: x.Redirect, DurationMS: int(x.Duration / 1e6),
		})
	}
	sort.Slice(rep.Results, func(i, j int) bool { return rep.Results[i].Path < rep.Results[j].Path })
	return json.Marshal(rep)
}
