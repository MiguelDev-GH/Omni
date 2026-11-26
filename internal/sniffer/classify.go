package sniffer

import (
	"slices"

	"github.com/MiguelDev-GH/omni/internal/target"
)

// wellKnown mapeia portas conhecidas ao protocolo exibido nos gráficos.
var wellKnown = map[uint16]string{
	80:   "HTTP",
	443:  "HTTPS",
	53:   "DNS",
	5432: "PostgreSQL",
	3306: "MySQL",
	25:   "SMTP",
	465:  "SMTP",
	587:  "SMTP",
}

// PortRoles são as portas dos alvos por papel.
type PortRoles struct {
	App  []int // aplicação (HTTP)
	DNS  []int // servidor DNS
	SMTP []int // servidor SMTP
}

// RolesFor devolve as portas de aplicação, DNS e SMTP de todos os alvos.
func RolesFor(ts []target.Target) PortRoles {
	var r PortRoles
	for _, t := range ts {
		r.App = append(r.App, t.AppPorts...)
		if t.DNSPort > 0 {
			r.DNS = append(r.DNS, t.DNSPort)
		}
		if t.SMTPPort > 0 {
			r.SMTP = append(r.SMTP, t.SMTPPort)
		}
	}
	return r
}

// Classify nomeia o protocolo de um pacote pelas portas. As portas dos alvos
// valem antes das conhecidas: as de aplicação contam como HTTP, as de DNS e
// SMTP pelo nome do serviço. Sem porta conhecida, devolve "TCP" ou "UDP".
func Classify(srcPort, dstPort uint16, isUDP bool, roles PortRoles) string {
	for _, p := range []uint16{dstPort, srcPort} {
		switch {
		case slices.Contains(roles.App, int(p)):
			return "HTTP"
		case slices.Contains(roles.DNS, int(p)):
			return "DNS"
		case slices.Contains(roles.SMTP, int(p)):
			return "SMTP"
		}
	}
	for _, p := range []uint16{dstPort, srcPort} {
		if name, ok := wellKnown[p]; ok {
			return name
		}
	}
	if isUDP {
		return "UDP"
	}
	return "TCP"
}
