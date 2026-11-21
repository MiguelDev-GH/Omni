package sniffer

import "slices"

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

// Classify nomeia o protocolo de um pacote pelas portas. As portas de
// aplicação dos alvos contam como HTTP; sem porta conhecida, devolve "TCP" ou "UDP".
func Classify(srcPort, dstPort uint16, isUDP bool, appPorts []int) string {
	for _, p := range []uint16{dstPort, srcPort} {
		if slices.Contains(appPorts, int(p)) {
			return "HTTP"
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
