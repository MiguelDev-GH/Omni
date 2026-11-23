package arsenal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// AmassImage é a imagem oficial do Amass, com versão fixa: a 3.x é a última
// que escreve o resultado em JSON (a 4.x só grava texto e o banco de grafos).
const AmassImage = "caffix/amass:v3.23.3"

// AmassArgs devolve os argumentos do Amass em modo passivo para o domínio do
// alvo: ele só consulta fontes públicas na internet e não manda nada ao
// alvo, por isso o host do alvo nunca entra nos argumentos. -timeout (em
// minutos) encerra o Amass antes dos 10 minutos da varredura.
func AmassArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	if t.Domain == "" {
		return nil, &target.ValidationError{Field: "dominio", Message: "o Amass precisa do domínio do alvo"}
	}
	if err := noOptions("amass", o); err != nil {
		return nil, err
	}
	return []string{"enum", "-passive", "-d", t.Domain, "-silent", "-nocolor", "-json", "/dev/stdout", "-timeout", "8"}, nil
}

// AmassReport são os subdomínios que o Amass encontrou para o domínio.
type AmassReport struct {
	Domain     string      `json:"dominio"`
	Subdomains []Subdomain `json:"subdominios"`
}

type Subdomain struct {
	Name      string   `json:"nome"`
	Addresses []string `json:"enderecos"`
	Sources   []string `json:"fontes"`
	// ThisHost diz que algum endereço é de loopback ou igual ao host do
	// alvo, ou seja, que o subdomínio aponta para este servidor.
	ThisHost bool `json:"neste_servidor"`
}

type amassLine struct {
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	Addresses []struct {
		IP string `json:"ip"`
	} `json:"addresses"`
	Sources []string `json:"sources"`
}

// amassSubdomains lê as linhas JSON do Amass e junta as repetidas pelo nome
// (ele repete um nome quando acha outra fonte ou endereço), em ordem
// alfabética. host é o Host do alvo, para marcar ThisHost; vazio não marca
// nada além do loopback.
func amassSubdomains(r io.Reader, host string) (domain string, subs []Subdomain, err error) {
	byName := map[string]*Subdomain{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var l amassLine
		if err := json.Unmarshal(line, &l); err != nil {
			return "", nil, fmt.Errorf("ler JSON do amass: %w", err)
		}
		name := strings.ToLower(strings.TrimSuffix(l.Name, "."))
		if name == "" {
			continue
		}
		if domain == "" {
			domain = l.Domain
		}
		s, ok := byName[name]
		if !ok {
			s = &Subdomain{Name: name, Addresses: []string{}, Sources: []string{}}
			byName[name] = s
		}
		for _, a := range l.Addresses {
			s.Addresses = appendNew(s.Addresses, a.IP)
			if ip := net.ParseIP(a.IP); ip != nil && (ip.IsLoopback() || a.IP == host) {
				s.ThisHost = true
			}
		}
		for _, src := range l.Sources {
			s.Sources = appendNew(s.Sources, src)
		}
	}
	if err := sc.Err(); err != nil {
		return "", nil, err
	}
	subs = make([]Subdomain, 0, len(byName))
	for _, s := range byName {
		subs = append(subs, *s)
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name < subs[j].Name })
	return domain, subs, nil
}

func appendNew(l []string, v string) []string {
	if v == "" {
		return l
	}
	for _, x := range l {
		if x == v {
			return l
		}
	}
	return append(l, v)
}

// ParseAmassJSON devolve um achado info por subdomínio. No modo passivo o
// Amass não resolve nomes, então a falta de endereço é a regra e não vira
// alerta de takeover.
func ParseAmassJSON(r io.Reader) ([]findings.Finding, error) {
	_, subs, err := amassSubdomains(r, "")
	if err != nil {
		return nil, err
	}
	list := make([]findings.Finding, 0, len(subs))
	for _, s := range subs {
		evidence := "fontes: " + strings.Join(s.Sources, ", ")
		if len(s.Addresses) > 0 {
			evidence += "; endereços: " + strings.Join(s.Addresses, ", ")
		}
		list = append(list, findings.New("amass", "Subdomínio encontrado: "+truncate(s.Name, maxEvidence),
			truncate(evidence, maxEvidence), findings.Info, 0))
	}
	return list, nil
}

// AmassReportFrom monta o AmassReport; o alvo marca os subdomínios que
// apontam para este servidor e dá o domínio quando nada foi encontrado.
func AmassReportFrom(t target.Target, out []byte) (json.RawMessage, error) {
	domain, subs, err := amassSubdomains(bytes.NewReader(out), t.Host)
	if err != nil {
		return nil, err
	}
	if domain == "" {
		domain = t.Domain
	}
	return json.Marshal(AmassReport{Domain: domain, Subdomains: subs})
}
