// Package orchestrator roda as ferramentas do Arsenal em containers Docker
// efêmeros e grava as varreduras e os achados.
package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MiguelDev-GH/omni/internal/arsenal"
	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/store"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

const (
	EventStarted = "SCAN_INICIADO"
	EventLog     = "SCAN_LOG"
	EventDone    = "SCAN_CONCLUIDO"
	EventFailed  = "SCAN_FALHOU"
	// EventEngines leva o estado do Docker, como {"docker": "ativo"}.
	EventEngines = "ESTADO_MOTORES"

	StateActive      = "ativo"
	StateUnavailable = "indisponivel"

	defaultTimeout  = 10 * time.Minute
	defaultLogEvery = 500 * time.Millisecond
	maxLineLen      = 2000
	maxLogLines     = 20_000
)

var (
	ErrDockerIndisponivel     = errors.New("docker indisponível")
	ErrFerramentaDesconhecida = errors.New("ferramenta desconhecida")
	ErrSemFerramentas         = errors.New("escolha ao menos uma ferramenta")
	ErrVarreduraEmAndamento   = errors.New("já existe uma varredura em andamento para este alvo")
)

// Spec descreve um container: imagem com versão fixa e argumentos em lista.
type Spec struct {
	Image   string
	Args    []string
	Volumes map[string]string
	// Files são copiados para o container antes de ele iniciar (caminho
	// absoluto → conteúdo), somente leitura.
	Files map[string][]byte
	// OutputFile é lido do container depois que ele termina e acrescentado
	// à saída padrão depois de arsenal.OutputFileMarker; ausente, é ignorado.
	OutputFile string
	AnyExit    bool
	// Memory é o limite de memória em bytes; zero usa o padrão de 512 MiB.
	Memory int64
}

// Runner roda um container até ele terminar e devolve a saída padrão.
type Runner interface {
	Run(ctx context.Context, s Spec, logs io.Writer) (stdout []byte, err error)
	Available(ctx context.Context) error
}

// Store é o que o orquestrador usa do banco.
type Store interface {
	GetTarget(ctx context.Context, id int64) (target.Target, error)
	CreateScan(ctx context.Context, targetID int64, tools []string, opts map[string]map[string]string) (store.Scan, error)
	SaveReport(ctx context.Context, scanID int64, tool string, report json.RawMessage) error
	UpdateScanStatus(ctx context.Context, id int64, status, errMsg string) error
	AddFindings(ctx context.Context, scanID int64, list []findings.Finding) error
	ScanFindings(ctx context.Context, scanID int64) ([]findings.Finding, error)
	PreviousScan(ctx context.Context, targetID, beforeID int64, tool string) (store.Scan, bool, error)
}

// Orchestrator dispara varreduras em segundo plano, no máximo uma por alvo.
type Orchestrator struct {
	ctx      context.Context
	runner   Runner
	st       Store
	pub      ws.Publisher
	timeout  time.Duration
	logEvery time.Duration

	mu      sync.Mutex
	running map[int64]bool // alvos com varredura em andamento
	docker  string
	wg      sync.WaitGroup
}

// New cria o orquestrador; as varreduras param quando ctx é cancelado.
func New(ctx context.Context, r Runner, st Store, pub ws.Publisher) *Orchestrator {
	return &Orchestrator{
		ctx: ctx, runner: r, st: st, pub: pub,
		timeout: defaultTimeout, logEvery: defaultLogEvery,
		running: map[int64]bool{},
	}
}

// Wait espera as varreduras em andamento terminarem.
func (o *Orchestrator) Wait() { o.wg.Wait() }

type job struct {
	tool   arsenal.Tool
	spec   Spec
	target target.Target
}

// Start valida o pedido, confirma que o Docker responde, grava a varredura
// e a roda em segundo plano. opts traz as opções de cada ferramenta pedida.
// Erros de validação vêm como ErrFerramentaDesconhecida, ErrSemFerramentas,
// *arsenal.OptionError, *target.ValidationError ou arsenal.ErrForaDoEscopo;
// alvo inexistente vem como store.ErrNotFound.
func (o *Orchestrator) Start(ctx context.Context, targetID int64, tools []string, opts map[string]arsenal.Options) (int64, error) {
	tools = dedupe(tools)
	if len(tools) == 0 {
		return 0, ErrSemFerramentas
	}
	var jobs []job
	for _, name := range tools {
		tool, ok := arsenal.Lookup(name)
		if !ok {
			return 0, fmt.Errorf("%w: %q", ErrFerramentaDesconhecida, name)
		}
		jobs = append(jobs, job{tool: tool})
	}
	for name := range opts {
		if !slices.Contains(tools, name) {
			return 0, &arsenal.OptionError{Tool: name}
		}
	}
	t, err := o.st.GetTarget(ctx, targetID)
	if err != nil {
		return 0, err
	}
	saved := map[string]map[string]string{}
	for i := range jobs {
		name := jobs[i].tool.Name
		args, err := jobs[i].tool.Args(t, opts[name])
		if err != nil {
			return 0, err
		}
		if len(opts[name]) > 0 {
			saved[name] = opts[name]
		}
		tool := jobs[i].tool
		var files map[string][]byte
		if tool.Files != nil {
			if files, err = tool.Files(opts[name]); err != nil {
				return 0, err
			}
		}
		jobs[i].target = t
		jobs[i].spec = Spec{Image: tool.Image, Args: args, Volumes: tool.Volumes, Files: files,
			OutputFile: tool.OutputFile, AnyExit: tool.AnyExit, Memory: tool.Memory}
	}
	if err := o.runner.Available(ctx); err != nil {
		o.setDocker(StateUnavailable)
		if !errors.Is(err, ErrDockerIndisponivel) {
			err = fmt.Errorf("%w: %v", ErrDockerIndisponivel, err)
		}
		return 0, err
	}
	o.setDocker(StateActive)

	o.mu.Lock()
	if o.running[targetID] {
		o.mu.Unlock()
		return 0, ErrVarreduraEmAndamento
	}
	o.running[targetID] = true
	o.mu.Unlock()

	sc, err := o.st.CreateScan(ctx, targetID, tools, saved)
	if err != nil {
		o.release(targetID)
		return 0, err
	}
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		defer o.release(targetID)
		o.run(sc, jobs)
	}()
	return sc.ID, nil
}

func (o *Orchestrator) release(targetID int64) {
	o.mu.Lock()
	delete(o.running, targetID)
	o.mu.Unlock()
}

func (o *Orchestrator) run(sc store.Scan, jobs []job) {
	ctx := o.ctx
	if err := o.st.UpdateScanStatus(ctx, sc.ID, store.ScanRunning, ""); err != nil {
		slog.Error("arsenal: marcar varredura como rodando", "erro", err)
	}
	o.pub.Publish(ws.Event{Type: EventStarted, Data: map[string]any{"scan_id": sc.ID, "alvo_id": sc.TargetID, "ferramentas": sc.Tools}})

	var failures []string
	total := 0
	for _, j := range jobs {
		n, err := o.runTool(ctx, sc, j)
		total += n
		if err != nil {
			failures = append(failures, j.tool.Name+": "+err.Error())
		}
	}

	// O estado final é gravado mesmo durante o desligamento.
	final, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		failures = append(failures, "interrompida: o Omni foi encerrado durante a varredura")
	}
	if len(failures) > 0 {
		msg := strings.Join(failures, "; ")
		if err := o.st.UpdateScanStatus(final, sc.ID, store.ScanFailed, msg); err != nil {
			slog.Error("arsenal: marcar varredura como falha", "erro", err)
		}
		o.pub.Publish(ws.Event{Type: EventFailed, Data: map[string]any{"scan_id": sc.ID, "alvo_id": sc.TargetID, "erro": msg, "achados": total}})
		return
	}
	if err := o.st.UpdateScanStatus(final, sc.ID, store.ScanDone, ""); err != nil {
		slog.Error("arsenal: marcar varredura como concluída", "erro", err)
	}
	o.pub.Publish(ws.Event{Type: EventDone, Data: map[string]any{
		"scan_id": sc.ID, "alvo_id": sc.TargetID, "ferramentas": sc.Tools, "achados": total,
		"novos_desde_ultimo": o.newSinceLast(final, sc),
	}})
}

// runTool roda uma ferramenta e grava os achados e o relatório dela; devolve
// quantos achados foram. Falha ao montar o relatório não falha a ferramenta:
// os achados ficam gravados e o erro vai para a saída ao vivo.
func (o *Orchestrator) runTool(ctx context.Context, sc store.Scan, j job) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()
	logs := newLogBatcher(o.pub, sc.ID, j.tool.Name, o.logEvery)
	defer logs.Close()
	_, _ = fmt.Fprintf(logs, "$ %s %s\n", j.tool.Name, strings.Join(j.spec.Args, " "))
	stdout, err := o.runner.Run(ctx, j.spec, logs)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return 0, fmt.Errorf("tempo limite de %s esgotado", o.timeout)
		}
		return 0, err
	}
	list, err := j.tool.Parse(bytes.NewReader(stdout))
	if err != nil {
		return 0, err
	}
	if err := o.st.AddFindings(ctx, sc.ID, list); err != nil {
		return 0, err
	}
	if j.tool.Report != nil {
		if err := o.saveReport(ctx, sc.ID, j, stdout); err != nil {
			slog.Warn("arsenal: relatório da ferramenta", "ferramenta", j.tool.Name, "erro", err)
			logs.Line("relatório da ferramenta indisponível: " + err.Error())
		}
	}
	return len(list), nil
}

func (o *Orchestrator) saveReport(ctx context.Context, scanID int64, j job, stdout []byte) error {
	report, err := j.tool.Report(j.target, stdout)
	if err != nil {
		return err
	}
	return o.st.SaveReport(ctx, scanID, j.tool.Name, report)
}

// newSinceLast conta os achados da varredura que não existiam na varredura
// concluída anterior de cada ferramenta.
func (o *Orchestrator) newSinceLast(ctx context.Context, sc store.Scan) int {
	cmp, err := Compare(ctx, o.st, sc)
	if err != nil {
		slog.Error("arsenal: comparar com a varredura anterior", "erro", err)
		return 0
	}
	return len(cmp.New)
}

// Comparison são os achados novos e resolvidos de uma varredura em relação
// à anterior de cada ferramenta.
type Comparison struct {
	New      []findings.Finding `json:"novos"`
	Resolved []findings.Finding `json:"resolvidos"`
	// Previous mapeia cada ferramenta ao ID da varredura usada na
	// comparação; ferramenta sem varredura anterior fica de fora.
	Previous map[string]int64 `json:"anteriores"`
}

// Compare compara sc com a última varredura concluída do mesmo alvo para
// cada ferramenta. Sem varredura anterior, todos os achados da ferramenta são novos.
func Compare(ctx context.Context, st Store, sc store.Scan) (Comparison, error) {
	cmp := Comparison{New: []findings.Finding{}, Resolved: []findings.Finding{}, Previous: map[string]int64{}}
	curr, err := st.ScanFindings(ctx, sc.ID)
	if err != nil {
		return cmp, err
	}
	for _, tool := range sc.Tools {
		mine := byTool(curr, tool)
		var before []findings.Finding
		prev, ok, err := st.PreviousScan(ctx, sc.TargetID, sc.ID, tool)
		if err != nil {
			return cmp, err
		}
		if ok {
			cmp.Previous[tool] = prev.ID
			all, err := st.ScanFindings(ctx, prev.ID)
			if err != nil {
				return cmp, err
			}
			before = byTool(all, tool)
		}
		novos, resolvidos := findings.Diff(before, mine)
		cmp.New = append(cmp.New, novos...)
		cmp.Resolved = append(cmp.Resolved, resolvidos...)
	}
	findings.Sort(cmp.New)
	findings.Sort(cmp.Resolved)
	return cmp, nil
}

func byTool(list []findings.Finding, tool string) []findings.Finding {
	var out []findings.Finding
	for _, f := range list {
		if f.Tool == tool {
			out = append(out, f)
		}
	}
	return out
}

// MonitorDocker publica o estado do Docker a cada every, até ctx acabar.
func (o *Orchestrator) MonitorDocker(ctx context.Context, every time.Duration) {
	for {
		if err := o.runner.Available(ctx); err != nil {
			o.setDocker(StateUnavailable)
		} else {
			o.setDocker(StateActive)
		}
		o.publishDocker()
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

// DockerState devolve "ativo", "indisponivel" ou "" antes da primeira verificação.
func (o *Orchestrator) DockerState() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.docker
}

func (o *Orchestrator) setDocker(s string) {
	o.mu.Lock()
	changed := o.docker != s
	o.docker = s
	o.mu.Unlock()
	if changed {
		slog.Info("docker " + s)
		o.publishDocker()
	}
}

func (o *Orchestrator) publishDocker() {
	if s := o.DockerState(); s != "" {
		o.pub.Publish(ws.Event{Type: EventEngines, Data: map[string]string{"docker": s}})
	}
}

func dedupe(list []string) []string {
	var out []string
	for _, s := range list {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
