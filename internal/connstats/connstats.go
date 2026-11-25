// Package connstats conta as conexões TCP abertas nas portas dos alvos e o
// pool de cada processo cliente, lidos do sistema (/proc no Linux).
package connstats

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

// EventConnections é o tipo do evento publicado a cada coleta.
const EventConnections = "METRICAS_CONEXOES"

// DefaultInterval é o intervalo padrão entre coletas.
const DefaultInterval = 5 * time.Second

// Papéis de uma porta.
const (
	KindApp  = "app"
	KindDB   = "banco"
	KindSMTP = "smtp"
)

const (
	// maxClients limita os processos clientes listados por porta.
	maxClients  = 10
	pruneEvery  = time.Minute
	established = "ESTABLISHED"
	listening   = "LISTEN"
)

// PortRole é uma porta de um alvo e o papel dela.
type PortRole struct {
	Port int
	Kind string
}

// ClientCount é o pool de conexões de um processo cliente.
type ClientCount struct {
	Process string `json:"processo"` // "" quando o Omni não pode ver o processo
	Conns   int    `json:"conexoes"`
}

// PortConns resume as conexões de uma porta.
type PortConns struct {
	Port        int            `json:"porta"`
	Kind        string         `json:"tipo"`
	Established int            `json:"estabelecidas"`
	States      map[string]int `json:"estados"`
	Clients     []ClientCount  `json:"clientes"`
}

// Snapshot é uma coleta de todas as portas.
type Snapshot struct {
	At    time.Time   `json:"hora"`
	Ports []PortConns `json:"portas"`
}

// Roles devolve as portas TCP dos alvos com o papel de cada uma, sem
// repetição (o primeiro papel vence). A porta DNS fica de fora: é UDP.
func Roles(ts []target.Target) []PortRole {
	seen := map[int]bool{}
	var roles []PortRole
	add := func(p int, kind string) {
		if p > 0 && !seen[p] {
			seen[p] = true
			roles = append(roles, PortRole{Port: p, Kind: kind})
		}
	}
	for _, t := range ts {
		for _, p := range t.AppPorts {
			add(p, KindApp)
		}
		add(t.DBPort, KindDB)
		add(t.SMTPPort, KindSMTP)
	}
	return roles
}

// Summarize conta, para cada papel, as conexões do lado servidor (porta
// local igual à porta do papel) por estado e agrupa as do lado cliente
// (porta remota igual, ESTABLISHED) pelo nome do processo. O socket em
// LISTEN não é uma conexão e fica de fora.
func Summarize(conns []gnet.ConnectionStat, roles []PortRole, names func(pid int32) string) []PortConns {
	out := make([]PortConns, 0, len(roles))
	for _, r := range roles {
		pc := PortConns{Port: r.Port, Kind: r.Kind, States: map[string]int{}, Clients: []ClientCount{}}
		pools := map[string]int{}
		for _, c := range conns {
			switch {
			case int(c.Laddr.Port) == r.Port && c.Status != listening && c.Status != "":
				pc.States[c.Status]++
				if c.Status == established {
					pc.Established++
				}
			case int(c.Raddr.Port) == r.Port && c.Status == established:
				name := ""
				if c.Pid > 0 {
					name = names(c.Pid)
				}
				pools[name]++
			}
		}
		for name, n := range pools {
			pc.Clients = append(pc.Clients, ClientCount{Process: name, Conns: n})
		}
		sort.Slice(pc.Clients, func(i, j int) bool {
			a, b := pc.Clients[i], pc.Clients[j]
			if a.Conns != b.Conns {
				return a.Conns > b.Conns
			}
			return a.Process < b.Process
		})
		if len(pc.Clients) > maxClients {
			pc.Clients = pc.Clients[:maxClients]
		}
		out = append(out, pc)
	}
	return out
}

// Sink guarda as coletas para o histórico.
type Sink interface {
	AddConnSnapshot(ctx context.Context, s Snapshot) error
	// PruneConnections apaga as coletas anteriores a before.
	PruneConnections(ctx context.Context, before time.Time) error
}

// Run coleta as conexões das portas dos alvos a cada interval até ctx ser
// cancelado, grava no sink e publica. Falhas são registradas e o ciclo segue.
// Uma vez por minuto, apaga as coletas mais antigas que retention.
func Run(ctx context.Context, pub ws.Publisher, sink Sink, targets func(context.Context) ([]target.Target, error), interval, retention time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastPrune time.Time
	for {
		snap, err := sample(ctx, targets)
		switch {
		case err == nil:
			if err := sink.AddConnSnapshot(ctx, snap); err != nil && ctx.Err() == nil {
				slog.Error("gravar conexões", "erro", err)
			}
			pub.Publish(ws.Event{Type: EventConnections, Data: snap})
			if snap.At.Sub(lastPrune) >= pruneEvery {
				lastPrune = snap.At
				if err := sink.PruneConnections(ctx, snap.At.Add(-retention)); err != nil && ctx.Err() == nil {
					slog.Error("apagar conexões antigas", "erro", err)
				}
			}
		case !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded):
			slog.Error("coletar conexões", "erro", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func sample(ctx context.Context, targets func(context.Context) ([]target.Target, error)) (Snapshot, error) {
	ts, err := targets(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	roles := Roles(ts)
	snap := Snapshot{At: time.Now().UTC(), Ports: []PortConns{}}
	if len(roles) == 0 {
		return snap, nil
	}
	conns, err := gnet.ConnectionsWithContext(ctx, "tcp")
	if err != nil {
		return Snapshot{}, err
	}
	// Os nomes valem só nesta coleta: um PID pode ser reaproveitado depois.
	cache := map[int32]string{}
	names := func(pid int32) string {
		if name, ok := cache[pid]; ok {
			return name
		}
		name := ""
		if p, err := process.NewProcessWithContext(ctx, pid); err == nil {
			name, _ = p.NameWithContext(ctx)
		}
		cache[pid] = name
		return name
	}
	snap.Ports = Summarize(conns, roles, names)
	return snap, nil
}
