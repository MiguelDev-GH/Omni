package arsenal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MiguelDev-GH/omni/internal/findings"
)

// Os relatórios alimentam só a página de cada ferramenta; a comparação entre
// varreduras e os alertas continuam usando os achados.

// NmapReport são todas as portas varridas, inclusive fechadas e filtradas.
type NmapReport struct {
	Hosts []NmapHost `json:"hosts"`
}

type NmapHost struct {
	Address string     `json:"endereco"`
	Ports   []NmapPort `json:"portas"`
}

type NmapPort struct {
	Port     int    `json:"porta"`
	Protocol string `json:"protocolo"`
	// State é o estado do Nmap: open, closed ou filtered.
	State   string `json:"estado"`
	Service string `json:"servico"`
	Product string `json:"produto"`
	Version string `json:"versao"`
	Extra   string `json:"extra"`
}

type nmapReportRun struct {
	Hosts []struct {
		Addresses []struct {
			Addr string `xml:"addr,attr"`
			Type string `xml:"addrtype,attr"`
		} `xml:"address"`
		Ports []struct {
			Protocol string `xml:"protocol,attr"`
			PortID   int    `xml:"portid,attr"`
			State    struct {
				State string `xml:"state,attr"`
			} `xml:"state"`
			Service struct {
				Name      string `xml:"name,attr"`
				Product   string `xml:"product,attr"`
				Version   string `xml:"version,attr"`
				ExtraInfo string `xml:"extrainfo,attr"`
			} `xml:"service"`
		} `xml:"ports>port"`
	} `xml:"host"`
}

// NmapReportFrom monta o NmapReport a partir do XML do Nmap.
func NmapReportFrom(stdout []byte) (json.RawMessage, error) {
	var run nmapReportRun
	if err := xml.NewDecoder(bytes.NewReader(stdout)).Decode(&run); err != nil {
		return nil, fmt.Errorf("ler XML do nmap: %w", err)
	}
	rep := NmapReport{Hosts: []NmapHost{}}
	for _, h := range run.Hosts {
		host := NmapHost{Ports: []NmapPort{}}
		for _, a := range h.Addresses {
			if a.Type != "mac" {
				host.Address = a.Addr
				break
			}
		}
		for _, p := range h.Ports {
			host.Ports = append(host.Ports, NmapPort{
				Port: p.PortID, Protocol: p.Protocol, State: p.State.State, Service: p.Service.Name,
				Product: p.Service.Product, Version: p.Service.Version, Extra: p.Service.ExtraInfo,
			})
		}
		rep.Hosts = append(rep.Hosts, host)
	}
	return json.Marshal(rep)
}

// NucleiReport são os resultados dos templates, com os metadados de cada um.
type NucleiReport struct {
	Results []NucleiResult `json:"resultados"`
}

type NucleiResult struct {
	Template    string            `json:"template"`
	Name        string            `json:"nome"`
	Severity    findings.Severity `json:"severidade"`
	Tags        []string          `json:"tags"`
	URL         string            `json:"url"`
	Matcher     string            `json:"matcher"`
	Extracted   []string          `json:"extraidos"`
	Description string            `json:"descricao"`
	References  []string          `json:"referencias"`
}

// stringList aceita um texto ou uma lista de textos, como o campo
// reference dos templates do Nuclei.
type stringList []string

func (l *stringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		if one != "" {
			*l = stringList{one}
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*l = many
	return nil
}

// nucleiReportLine só declara os campos usados: request e response, que
// podem conter dados do alvo, nunca são lidos.
type nucleiReportLine struct {
	TemplateID string `json:"template-id"`
	Info       struct {
		Name        string     `json:"name"`
		Severity    string     `json:"severity"`
		Tags        stringList `json:"tags"`
		Description string     `json:"description"`
		Reference   stringList `json:"reference"`
	} `json:"info"`
	MatcherName string   `json:"matcher-name"`
	MatchedAt   string   `json:"matched-at"`
	Extracted   []string `json:"extracted-results"`
}

// NucleiReportFrom monta o NucleiReport a partir do JSONL do Nuclei. Como
// cada linha é independente, linhas inválidas ou cortadas são ignoradas, do
// mesmo jeito que em ParseNucleiJSONL.
func NucleiReportFrom(stdout []byte) (json.RawMessage, error) {
	rep := NucleiReport{Results: []NucleiResult{}}
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var res nucleiReportLine
		if err := json.Unmarshal([]byte(line), &res); err != nil || res.TemplateID == "" {
			continue
		}
		rep.Results = append(rep.Results, NucleiResult{
			Template: res.TemplateID, Name: res.Info.Name, Severity: findings.FromTool(res.Info.Severity),
			Tags: nonNil(res.Info.Tags), URL: res.MatchedAt, Matcher: res.MatcherName,
			Extracted: nonNil(res.Extracted), Description: strings.TrimSpace(res.Info.Description),
			References: nonNil(res.Info.Reference),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(rep)
}

func nonNil(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

// TestsslReport é o boletim TLS: nota, protocolos, certificado e
// vulnerabilidades testadas.
type TestsslReport struct {
	// Grade é a nota geral (A+ a F, ou T para certificado não confiável);
	// vazia quando o testssl não deu nota.
	Grade       string         `json:"nota"`
	Protocols   []TLSProtocol  `json:"protocolos"`
	Certificate TLSCertificate `json:"certificado"`
	Vulns       []TLSVuln      `json:"vulnerabilidades"`
}

type TLSProtocol struct {
	Name    string `json:"nome"`
	Offered bool   `json:"oferecido"`
}

type TLSCertificate struct {
	Subject  string     `json:"assunto"`
	Issuer   string     `json:"emissor"`
	NotAfter *time.Time `json:"valido_ate"`
	DaysLeft int        `json:"dias_restantes"`
	Trusted  bool       `json:"confiavel"`
}

type TLSVuln struct {
	ID         string `json:"id"`
	Name       string `json:"nome"`
	Vulnerable bool   `json:"vulneravel"`
	CVE        string `json:"cve"`
}

// testsslProtocols são os IDs dos protocolos no JSON do testssl, na ordem
// exibida, com o nome mostrado na interface.
var testsslProtocols = []struct{ id, name string }{
	{"SSLv2", "SSLv2"}, {"SSLv3", "SSLv3"}, {"TLS1", "TLS 1.0"},
	{"TLS1_1", "TLS 1.1"}, {"TLS1_2", "TLS 1.2"}, {"TLS1_3", "TLS 1.3"},
}

// testsslVulns são as vulnerabilidades testadas pelo testssl, na ordem
// exibida, com o nome mostrado na interface.
var testsslVulns = []struct{ id, name string }{
	{"heartbleed", "Heartbleed"}, {"CCS", "CCS injection"}, {"ticketbleed", "Ticketbleed"},
	{"ROBOT", "ROBOT"}, {"secure_renego", "Renegociação segura"},
	{"secure_client_renego", "Renegociação iniciada pelo cliente"}, {"CRIME_TLS", "CRIME"},
	{"BREACH", "BREACH"}, {"POODLE_SSL", "POODLE"}, {"fallback_SCSV", "TLS_FALLBACK_SCSV"},
	{"SWEET32", "SWEET32"}, {"FREAK", "FREAK"}, {"DROWN", "DROWN"}, {"LOGJAM", "Logjam"},
	{"BEAST", "BEAST"}, {"LUCKY13", "Lucky13"}, {"winshock", "Winshock"}, {"RC4", "RC4"},
}

// testsslTime é o formato das datas do certificado no JSON do testssl.
const testsslTime = "2006-01-02 15:04"

// TestsslReportFrom monta o TestsslReport a partir da saída do testssl.
func TestsslReportFrom(stdout []byte) (json.RawMessage, error) {
	rep, err := testsslReport(stdout, time.Now())
	if err != nil {
		return nil, err
	}
	return json.Marshal(rep)
}

// testsslReport lê a saída do testssl; now calcula os dias restantes do
// certificado. A saída sem o objeto scanTime, que o testssl escreve por
// último, foi cortada e é recusada.
func testsslReport(stdout []byte, now time.Time) (TestsslReport, error) {
	byID := map[string]testsslResult{}
	for _, m := range testsslObject.FindAll(stdout, -1) {
		var res testsslResult
		if json.Unmarshal(m, &res) != nil || res.ID == "" {
			continue
		}
		if _, seen := byID[res.ID]; !seen {
			byID[res.ID] = res
		}
	}
	if _, ok := byID["scanTime"]; !ok {
		return TestsslReport{}, errors.New("saída do testssl incompleta")
	}
	finding := func(id string) string { return strings.Join(strings.Fields(byID[id].Finding), " ") }

	rep := TestsslReport{Grade: finding("overall_grade"), Protocols: []TLSProtocol{}, Vulns: []TLSVuln{}}
	for _, p := range testsslProtocols {
		if _, ok := byID[p.id]; !ok {
			continue
		}
		f := finding(p.id)
		rep.Protocols = append(rep.Protocols, TLSProtocol{Name: p.name, Offered: strings.HasPrefix(f, "offered")})
	}

	cert := TLSCertificate{Subject: finding("cert_commonName"), Issuer: finding("cert_caIssuers")}
	if r, ok := byID["cert_chain_of_trust"]; ok {
		cert.Trusted = r.Severity == "OK"
	}
	if t, err := time.Parse(testsslTime, finding("cert_notAfter")); err == nil {
		cert.NotAfter = &t
		cert.DaysLeft = int(t.Sub(now).Hours() / 24)
	}
	rep.Certificate = cert

	for _, v := range testsslVulns {
		r, ok := byID[v.id]
		if !ok {
			continue
		}
		rep.Vulns = append(rep.Vulns, TLSVuln{ID: v.id, Name: v.name, Vulnerable: vulnerable(r.Severity), CVE: r.CVE})
	}
	return rep, nil
}

func vulnerable(severity string) bool {
	switch severity {
	case "LOW", "MEDIUM", "HIGH", "CRITICAL":
		return true
	}
	return false
}
