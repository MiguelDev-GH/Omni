package arsenal

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

const (
	// NucleiImage é a imagem oficial do Nuclei, com versão fixa.
	NucleiImage = "projectdiscovery/nuclei:v3.4.10"
	// nucleiTemplates é o volume que guarda os templates entre varreduras.
	nucleiTemplates = "omni-nuclei-templates"
	maxEvidence     = 500
)

// NucleiArgs devolve os argumentos do Nuclei: um alvo http://<host>:<porta>
// por porta de aplicação, saída JSONL, templates HTTP de exposição, má
// configuração, tecnologias e CVEs, com limite de requisições e de
// concorrência para não sobrecarregar a produção e o User-Agent do Omni.
func NucleiArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	if err := noOptions("nuclei", o); err != nil {
		return nil, err
	}
	args := []string{"-jsonl", "-silent", "-nc", "-t", "http/", "-tags", "exposure,misconfig,tech,cve",
		"-rl", "100", "-c", "10", "-bs", "10", "-timeout", "5", "-retries", "1", "-H", "User-Agent: " + UserAgent}
	for _, p := range t.AppPorts {
		args = append(args, "-u", "http://"+net.JoinHostPort(t.Host, strconv.Itoa(p)))
	}
	return args, nil
}

type nucleiResult struct {
	TemplateID string `json:"template-id"`
	Info       struct {
		Name     string `json:"name"`
		Severity string `json:"severity"`
	} `json:"info"`
	MatcherName string   `json:"matcher-name"`
	MatchedAt   string   `json:"matched-at"`
	Port        string   `json:"port"`
	Extracted   []string `json:"extracted-results"`
}

// ParseNucleiJSONL lê uma Finding por linha JSON. Linhas que não são JSON
// válido (como a tabela de atualização dos templates) são ignoradas.
// Requisição e resposta, que podem conter dados do alvo, nunca são lidas.
func ParseNucleiJSONL(r io.Reader) ([]findings.Finding, error) {
	out := []findings.Finding{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var res nucleiResult
		if err := json.Unmarshal([]byte(line), &res); err != nil || res.TemplateID == "" {
			continue
		}
		title := res.Info.Name
		if title == "" {
			title = res.TemplateID
		}
		if res.MatcherName != "" {
			title += ": " + res.MatcherName
		}
		evidence := res.MatchedAt
		if len(res.Extracted) > 0 {
			evidence += " · " + strings.Join(res.Extracted, ", ")
		}
		port, _ := strconv.Atoi(res.Port)
		out = append(out, findings.New("nuclei", title, truncate(evidence, maxEvidence), findings.FromTool(res.Info.Severity), port))
	}
	return out, sc.Err()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
