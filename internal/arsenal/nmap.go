package arsenal

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// NmapImage é a imagem do Nmap, com versão fixa (Alpine, mantida pela
// instrumentisto).
const NmapImage = "instrumentisto/nmap:7.95"

// NmapArgs devolve os argumentos do Nmap para as portas do alvo: detecção de
// versão, sem ping e com o XML na saída padrão. O "--" impede que o host seja
// lido como opção.
func NmapArgs(t target.Target) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	ps := ports(t)
	list := make([]string, len(ps))
	for i, p := range ps {
		list[i] = strconv.Itoa(p)
	}
	return []string{"-sV", "-Pn", "-oX", "-", "-p", strings.Join(list, ","), "--", t.Host}, nil
}

type nmapRun struct {
	Hosts []struct {
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

// ParseNmapXML devolve uma Finding de severidade info por porta aberta, com
// título como "Porta 3000/tcp aberta: http (nginx 1.25)".
func ParseNmapXML(r io.Reader) ([]findings.Finding, error) {
	var run nmapRun
	if err := xml.NewDecoder(r).Decode(&run); err != nil {
		return nil, fmt.Errorf("ler XML do nmap: %w", err)
	}
	out := []findings.Finding{}
	for _, h := range run.Hosts {
		for _, p := range h.Ports {
			if p.State.State != "open" {
				continue
			}
			name := p.Service.Name
			if name == "" {
				name = "desconhecido"
			}
			title := fmt.Sprintf("Porta %d/%s aberta: %s", p.PortID, p.Protocol, name)
			product := strings.TrimSpace(p.Service.Product + " " + p.Service.Version)
			if product != "" {
				title += " (" + product + ")"
			}
			out = append(out, findings.New("nmap", title, p.Service.ExtraInfo, findings.Info, p.PortID))
		}
	}
	return out, nil
}
