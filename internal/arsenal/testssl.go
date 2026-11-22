package arsenal

import (
	"encoding/json"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// TestsslImage é a imagem oficial do testssl.sh, com versão fixa.
const TestsslImage = "drwetter/testssl.sh:3.2"

// TestsslArgs devolve os argumentos do testssl com o JSON na saída padrão. O
// alvo é o domínio na porta 443, ou o host na porta 443 (se for uma porta de
// aplicação) ou na primeira porta de aplicação.
func TestsslArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	if err := noOptions("testssl", o); err != nil {
		return nil, err
	}
	host, port := t.Host, t.AppPorts[0]
	switch {
	case t.Domain != "":
		host, port = t.Domain, 443
	case contains(t.AppPorts, 443):
		port = 443
	}
	return []string{"--quiet", "--color", "0", "--warnings", "off", "--connect-timeout", "10",
		"--openssl-timeout", "10", "--jsonfile", "/dev/stdout", net.JoinHostPort(host, strconv.Itoa(port))}, nil
}

// testsslObject acha os objetos do JSON do testssl, que são planos, mesmo no
// meio do texto do console que divide a saída padrão com eles.
var testsslObject = regexp.MustCompile(`\{[^{}]*\}`)

type testsslResult struct {
	ID       string `json:"id"`
	Port     string `json:"port"`
	Severity string `json:"severity"`
	Finding  string `json:"finding"`
	CVE      string `json:"cve"`
	CWE      string `json:"cwe"`
}

// ParseTestsslJSON devolve os problemas de severidade baixa ou maior e a
// nota geral (overall_grade); resultados OK, INFO e WARN (avisos sobre a
// própria execução) ficam de fora.
func ParseTestsslJSON(r io.Reader) ([]findings.Finding, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	out := []findings.Finding{}
	for _, m := range testsslObject.FindAll(b, -1) {
		var res testsslResult
		if json.Unmarshal(m, &res) != nil || res.ID == "" {
			continue
		}
		port, _ := strconv.Atoi(res.Port)
		finding := strings.Join(strings.Fields(res.Finding), " ")
		if res.ID == "overall_grade" {
			out = append(out, findings.New("testssl", "Nota TLS: "+finding, "", findings.FromTool(res.Severity), port))
			continue
		}
		switch res.Severity {
		case "LOW", "MEDIUM", "HIGH", "CRITICAL":
		default:
			continue
		}
		title := res.ID
		if finding != "" && finding != "--" {
			title += ": " + truncate(finding, 200)
		}
		evidence := strings.TrimSpace(res.CVE + " " + res.CWE)
		out = append(out, findings.New("testssl", title, evidence, findings.FromTool(res.Severity), port))
	}
	return out, nil
}

func contains(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
