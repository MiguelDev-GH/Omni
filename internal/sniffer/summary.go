package sniffer

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/MiguelDev-GH/omni/internal/sniffer/l7"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

// maxRequestLine limita a primeira linha lida de um pacote HTTP.
const maxRequestLine = 2048

// PacketSummary é uma linha do Inspetor. No HTTP, Info traz só o método e a
// rota normalizada (ou o status da resposta); no PostgreSQL, só o tipo da
// mensagem. Cabeçalhos, query string e corpo nunca entram.
type PacketSummary struct {
	Time     time.Time `json:"hora"`
	Src      string    `json:"origem"`
	Dst      string    `json:"destino"`
	SrcPort  int       `json:"porta_origem"`
	DstPort  int       `json:"porta_destino"`
	Protocol string    `json:"protocolo"`
	Length   int       `json:"tamanho"`
	Flags    string    `json:"flags"`
	Info     string    `json:"info"`
}

// Summarize resume p para o Inspetor. As portas classificam o protocolo como
// no resto do sniffer: roles.App é HTTP e dbPorts é PostgreSQL.
func Summarize(p gopacket.Packet, roles PortRoles, dbPorts []int) PacketSummary {
	s := PacketSummary{Time: p.Metadata().Timestamp.UTC(), Length: p.Metadata().Length, Protocol: "IP"}
	if s.Length == 0 {
		s.Length = len(p.Data())
	}
	switch ip := p.NetworkLayer().(type) {
	case *layers.IPv4:
		s.Src, s.Dst = ip.SrcIP.String(), ip.DstIP.String()
	case *layers.IPv6:
		s.Src, s.Dst = ip.SrcIP.String(), ip.DstIP.String()
	default:
		if l := p.LinkLayer(); l != nil {
			s.Protocol = l.LayerType().String()
		}
		return s
	}

	switch tr := p.TransportLayer().(type) {
	case *layers.TCP:
		s.SrcPort, s.DstPort = int(tr.SrcPort), int(tr.DstPort)
		s.Protocol = Classify(uint16(tr.SrcPort), uint16(tr.DstPort), false, roles)
		s.Flags = tcpFlags(tr)
		switch {
		case slices.Contains(roles.App, s.SrcPort) || slices.Contains(roles.App, s.DstPort):
			s.Info = httpInfo(tr.Payload)
		case slices.Contains(dbPorts, s.DstPort):
			s.Protocol = "PostgreSQL"
			s.Info = pgInfo(tr.Payload, true)
		case slices.Contains(dbPorts, s.SrcPort):
			s.Protocol = "PostgreSQL"
			s.Info = pgInfo(tr.Payload, false)
		}
	case *layers.UDP:
		s.SrcPort, s.DstPort = int(tr.SrcPort), int(tr.DstPort)
		s.Protocol = Classify(uint16(tr.SrcPort), uint16(tr.DstPort), true, roles)
	default:
		if p.Layer(layers.LayerTypeICMPv4) != nil || p.Layer(layers.LayerTypeICMPv6) != nil {
			s.Protocol = "ICMP"
		}
	}
	return s
}

// MatchesFilter diz se o pacote passa no filtro do Inspetor: IP e porta valem
// para a origem ou o destino; o protocolo ignora maiúsculas.
func (s PacketSummary) MatchesFilter(f ws.InspectorFilter) bool {
	if f.IP != "" && f.IP != s.Src && f.IP != s.Dst {
		return false
	}
	if f.Port != 0 && f.Port != s.SrcPort && f.Port != s.DstPort {
		return false
	}
	return f.Protocol == "" || strings.EqualFold(f.Protocol, s.Protocol)
}

func tcpFlags(t *layers.TCP) string {
	var fs []string
	for _, f := range []struct {
		on   bool
		name string
	}{{t.SYN, "SYN"}, {t.FIN, "FIN"}, {t.RST, "RST"}, {t.PSH, "PSH"}, {t.ACK, "ACK"}, {t.URG, "URG"}} {
		if f.on {
			fs = append(fs, f.name)
		}
	}
	return strings.Join(fs, ",")
}

// httpInfo lê só a primeira linha do payload: "POST /login" numa requisição,
// "HTTP 200" numa resposta, e vazio no resto (cabeçalhos ou corpo).
func httpInfo(payload []byte) string {
	line := payload
	if len(line) > maxRequestLine {
		line = line[:maxRequestLine]
	}
	i := bytes.IndexByte(line, '\n')
	if i < 0 {
		return ""
	}
	parts := strings.Fields(string(line[:i]))
	if len(parts) < 2 {
		return ""
	}
	if strings.HasPrefix(parts[0], "HTTP/") {
		if code, err := strconv.Atoi(parts[1]); err == nil && code >= 100 && code <= 999 {
			return "HTTP " + parts[1]
		}
		return ""
	}
	if len(parts) != 3 || !strings.HasPrefix(parts[2], "HTTP/") || !isMethod(parts[0]) {
		return ""
	}
	target := parts[1]
	if j := strings.Index(target, "://"); j >= 0 {
		rest := target[j+3:]
		if k := strings.IndexByte(rest, '/'); k >= 0 {
			target = rest[k:]
		} else {
			target = "/"
		}
	}
	return parts[0] + " " + l7.NormalizeRoute(target)
}

func isMethod(s string) bool {
	if s == "" || len(s) > 16 {
		return false
	}
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

var (
	pgFrontend = map[byte]string{
		'Q': "Query", 'P': "Parse", 'B': "Bind", 'E': "Execute", 'D': "Describe",
		'S': "Sync", 'X': "Terminate", 'C': "Close", 'H': "Flush", 'p': "PasswordMessage",
		'F': "FunctionCall", 'd': "CopyData", 'c': "CopyDone", 'f': "CopyFail",
	}
	pgBackend = map[byte]string{
		'R': "Authentication", 'S': "ParameterStatus", 'K': "BackendKeyData", 'Z': "ReadyForQuery",
		'T': "RowDescription", 'D': "DataRow", 'C': "CommandComplete", 'E': "ErrorResponse",
		'N': "NoticeResponse", '1': "ParseComplete", '2': "BindComplete", '3': "CloseComplete",
		'n': "NoData", 'I': "EmptyQueryResponse", 's': "PortalSuspended", 't': "ParameterDescription",
		'A': "NotificationResponse", 'G': "CopyInResponse", 'H': "CopyOutResponse", 'd': "CopyData",
		'c': "CopyDone",
	}
)

// pgInfo nomeia a primeira mensagem do PostgreSQL no payload, sem ler o conteúdo.
func pgInfo(payload []byte, fromClient bool) string {
	if fromClient && len(payload) >= 8 && binary.BigEndian.Uint32(payload) == uint32(len(payload)) {
		switch binary.BigEndian.Uint32(payload[4:]) {
		case 80877103:
			return "SSLRequest"
		case 80877104:
			return "GSSENCRequest"
		case 80877102:
			return "CancelRequest"
		case 196608:
			return "StartupMessage"
		}
	}
	if len(payload) < 5 || binary.BigEndian.Uint32(payload[1:]) < 4 {
		return ""
	}
	names := pgBackend
	if fromClient {
		names = pgFrontend
	}
	return names[payload[0]]
}
