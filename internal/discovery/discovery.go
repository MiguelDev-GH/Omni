// Package discovery lista os serviços que escutam portas TCP nesta máquina,
// para sugerir alvos no cadastro.
package discovery

import (
	"context"
	"net"
	"slices"
	"sort"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type Listener struct {
	Address string // IP local do socket, ex.: "127.0.0.1", "0.0.0.0", "::"
	Port    int
	PID     int32  // 0 quando o processo não é visível
	Process string // "" quando o processo não é visível
}

type Suggestion struct {
	Process string `json:"processo,omitempty"`
	PID     int32  `json:"pid,omitempty"`
	Host    string `json:"host"`
	Kind    string `json:"tipo"`   // "app" ou "banco"
	Ports   []int  `json:"portas"` // em ordem crescente
}

// Serviços do sistema que não costumam ser alvo de monitoramento.
var systemProcesses = map[string]bool{
	"sshd": true, "systemd-resolve": true, "systemd-resolved": true, "cupsd": true,
	"avahi-daemon": true, "chronyd": true, "rpcbind": true, "dnsmasq": true,
	"exim4": true, "master": true,
}

var dbProcesses = map[string]bool{
	"postgres": true, "mysqld": true, "mariadbd": true, "redis-server": true, "mongod": true,
}

var dbPorts = map[int]bool{5432: true, 3306: true, 6379: true, 27017: true}

// SystemListeners devolve os sockets TCP (IPv4 e IPv6) em estado LISTEN.
// Processos de outros usuários, invisíveis sem root, vêm sem PID e sem nome.
// A linha de comando dos processos nunca é lida: costuma conter segredos.
func SystemListeners(ctx context.Context) ([]Listener, error) {
	conns, err := gnet.ConnectionsWithContext(ctx, "tcp")
	if err != nil {
		return nil, err
	}
	names := map[int32]string{}
	var out []Listener
	for _, c := range conns {
		if c.Status != "LISTEN" {
			continue
		}
		l := Listener{Address: c.Laddr.IP, Port: int(c.Laddr.Port)}
		if c.Pid > 0 {
			name, ok := names[c.Pid]
			if !ok {
				name = processName(ctx, c.Pid)
				names[c.Pid] = name
			}
			if name != "" {
				l.PID, l.Process = c.Pid, name
			}
		}
		out = append(out, l)
	}
	return out, nil
}

func processName(ctx context.Context, pid int32) string {
	p, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return ""
	}
	name, err := p.NameWithContext(ctx)
	if err != nil {
		return ""
	}
	return name
}

// Suggest transforma os listeners em sugestões de alvo: descarta serviços do
// sistema e a porta do próprio Omni, agrupa por processo e separa aplicação
// de banco. Nunca devolve nil.
func Suggest(ls []Listener, ownPort int) []Suggestion {
	type key struct {
		pid  int32
		host string
		port int // só usado quando o PID é desconhecido
	}
	groups := map[key]*Suggestion{}
	var order []key
	for _, l := range ls {
		if l.Port == ownPort || systemProcesses[l.Process] {
			continue
		}
		host := hostFor(l.Address)
		k := key{pid: l.PID, host: host}
		if l.PID == 0 {
			k.port = l.Port
		}
		s, ok := groups[k]
		if !ok {
			s = &Suggestion{Process: l.Process, PID: l.PID, Host: host}
			groups[k] = s
			order = append(order, k)
		}
		if !slices.Contains(s.Ports, l.Port) {
			s.Ports = append(s.Ports, l.Port)
		}
	}

	out := make([]Suggestion, 0, len(order))
	for _, k := range order {
		s := groups[k]
		sort.Ints(s.Ports)
		s.Kind = kindOf(s)
		out = append(out, *s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind == "app"
		}
		return out[i].Ports[0] < out[j].Ports[0]
	})
	return out
}

// hostFor devolve o host que o Poller deve usar para alcançar o socket.
func hostFor(addr string) string {
	ip := net.ParseIP(addr)
	switch {
	case ip == nil || ip.IsUnspecified():
		return "127.0.0.1"
	case ip.Equal(net.IPv6loopback):
		return "::1"
	case ip.IsLoopback():
		return "127.0.0.1"
	default:
		return addr
	}
}

func kindOf(s *Suggestion) string {
	if dbProcesses[s.Process] {
		return "banco"
	}
	if s.Process == "" && dbPorts[s.Ports[0]] {
		return "banco"
	}
	return "app"
}
