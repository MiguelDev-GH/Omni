package capture

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"

	"github.com/MiguelDev-GH/omni/internal/sniffer"
	"github.com/MiguelDev-GH/omni/internal/target"
	"github.com/MiguelDev-GH/omni/internal/ws"
)

const (
	// EventStarted e EventFinished levam a Capture.
	EventStarted  = "CAPTURA_INICIADA"
	EventFinished = "CAPTURA_CONCLUIDA"

	MinDuration  = 10 * time.Second
	MaxDuration  = 300 * time.Second
	MaxFileBytes = 100 << 20
	MaxDirBytes  = 1 << 30
	// PageSize é quantos pacotes cada página do Inspetor em modo arquivo traz.
	PageSize = 500

	// bufferSize é quantos pacotes esperam a gravação; o excesso é descartado,
	// para o tap nunca travar a captura.
	bufferSize = 4096
	snaplen    = 65535
	// headerBytes e recordHeaderBytes são os cabeçalhos do formato pcap.
	headerBytes       = 24
	recordHeaderBytes = 16
	pruneEvery        = time.Minute
	finishTimeout     = 5 * time.Second
)

// Store é o que o gravador usa do banco.
type Store interface {
	CreateCapture(ctx context.Context, c Capture) (Capture, error)
	FinishCapture(ctx context.Context, c Capture) error
	FailInterruptedCaptures(ctx context.Context) error
	ListCaptures(ctx context.Context) ([]Capture, error)
	CapturesBefore(ctx context.Context, before time.Time) ([]Capture, error)
	GetCapture(ctx context.Context, id int64) (Capture, error)
	DeleteCapture(ctx context.Context, id int64) error
}

// Sniffer é a fonte dos pacotes: o motor do sniffer.
type Sniffer interface {
	AddRawTap(fn sniffer.RawTap) (remove func())
	State() string
}

// Targets lista os alvos, cujas portas classificam os pacotes no Inspetor.
type Targets func(ctx context.Context) ([]target.Target, error)

// FileName é o nome do arquivo da captura id.
func FileName(id int64) string { return "capture-" + strconv.FormatInt(id, 10) + ".pcap" }

// Recorder grava uma captura por vez, em arquivos com permissão 0600, e
// apaga as mais antigas que a retenção.
type Recorder struct {
	base      context.Context
	dir       string
	st        Store
	snif      Sniffer
	pub       ws.Publisher
	targets   Targets
	retention time.Duration
	maxFile   int64
	maxDir    int64
	// minDur é MinDuration; os testes o encurtam.
	minDur time.Duration

	mu     sync.Mutex
	active int64
	wg     sync.WaitGroup
}

// New cria o gravador. As gravações em andamento param quando ctx acaba;
// Wait espera por elas.
func New(ctx context.Context, dir string, st Store, snif Sniffer, pub ws.Publisher, targets Targets, retention time.Duration) *Recorder {
	return &Recorder{
		base: ctx, dir: dir, st: st, snif: snif, pub: pub, targets: targets, retention: retention,
		maxFile: MaxFileBytes, maxDir: MaxDirBytes, minDur: MinDuration,
	}
}

// Wait espera as gravações em andamento terminarem.
func (r *Recorder) Wait() { r.wg.Wait() }

// Start começa a gravar por d os pacotes que passam no filtro do Inspetor
// (vazio grava tudo o que o sniffer vê) e devolve o id da captura, sem
// esperar o fim.
func (r *Recorder) Start(ctx context.Context, d time.Duration, f ws.InspectorFilter) (int64, error) {
	if d < r.minDur || d > MaxDuration {
		return 0, ErrDuracaoInvalida
	}
	f, ok := ws.NormalizeFilter(f)
	if !ok {
		return 0, ErrFiltroInvalido
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != 0 {
		return 0, ErrEmAndamento
	}
	if r.snif.State() != sniffer.StateActive {
		return 0, ErrSnifferInativo
	}
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return 0, fmt.Errorf("criar o diretório das capturas: %w", err)
	}
	used, err := dirSize(r.dir)
	if err != nil {
		return 0, fmt.Errorf("medir o diretório das capturas: %w", err)
	}
	if used >= r.maxDir {
		return 0, ErrDiretorioCheio
	}
	// O protocolo de cada pacote sai das portas dos alvos, como no Inspetor.
	ts, err := r.targets(ctx)
	if err != nil {
		return 0, err
	}
	c, err := r.st.CreateCapture(ctx, Capture{
		StartedAt: time.Now(), Duration: int(d / time.Second), IP: f.IP, Port: f.Port, Protocol: f.Protocol,
	})
	if err != nil {
		return 0, err
	}
	file, err := os.OpenFile(filepath.Join(r.dir, c.File), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		c.Status, c.Error = StatusFailed, "não foi possível criar o arquivo"
		r.finish(c, false)
		return 0, fmt.Errorf("criar o arquivo da captura: %w", err)
	}
	rec := &recording{filter: f, roles: sniffer.RolesFor(ts), db: sniffer.DBPorts(ts), ch: make(chan packet, bufferSize)}
	r.active = c.ID
	remove := r.snif.AddRawTap(rec.tap)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.record(c, file, rec, remove, d, min(r.maxFile, r.maxDir-used))
	}()
	r.pub.Publish(ws.Event{Type: EventStarted, Data: c})
	return c.ID, nil
}

type packet struct {
	ci   gopacket.CaptureInfo
	data []byte
	link layers.LinkType
}

// recording recebe os pacotes do tap, na goroutine da captura.
type recording struct {
	filter  ws.InspectorFilter
	roles   sniffer.PortRoles
	db      []int
	ch      chan packet
	dropped atomic.Int64
}

func (rec *recording) tap(p gopacket.Packet, link layers.LinkType) {
	if rec.filter != (ws.InspectorFilter{}) && !sniffer.Summarize(p, rec.roles, rec.db).MatchesFilter(rec.filter) {
		return
	}
	ci := p.Metadata().CaptureInfo
	data := p.Data()
	ci.CaptureLength = len(data)
	if ci.Length < len(data) {
		ci.Length = len(data)
	}
	select {
	case rec.ch <- packet{ci: ci, data: data, link: link}:
	default:
		rec.dropped.Add(1)
	}
}

// record grava até d passar, o arquivo chegar a limit bytes ou o Omni
// encerrar. O cabeçalho do arquivo sai com o primeiro pacote, que traz o
// tipo de enlace; sem pacotes, o arquivo fica com cabeçalho Ethernet.
func (r *Recorder) record(c Capture, f *os.File, rec *recording, remove func(), d time.Duration, limit int64) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	buf := bufio.NewWriter(f)
	w := pcapgo.NewWriter(buf)
	var (
		link    layers.LinkType
		header  bool
		size    int64
		mixed   int64
		failure error
	)
	writeHeader := func(l layers.LinkType) {
		link, header, size = l, true, headerBytes
		failure = w.WriteFileHeader(snaplen, l)
	}
loop:
	for failure == nil {
		select {
		case <-r.base.Done():
			break loop
		case <-timer.C:
			break loop
		case p := <-rec.ch:
			if !header {
				writeHeader(p.link)
				if failure != nil {
					break loop
				}
			}
			if p.link != link {
				// Um arquivo pcap só tem um tipo de enlace.
				mixed++
				continue
			}
			n := int64(recordHeaderBytes + len(p.data))
			if size+n > limit {
				break loop
			}
			if failure = w.WritePacket(p.ci, p.data); failure == nil {
				size += n
				c.Packets++
			}
		}
	}
	remove()
	if failure == nil && !header {
		writeHeader(layers.LinkTypeEthernet)
	}
	if failure == nil {
		failure = buf.Flush()
	}
	if err := f.Close(); failure == nil {
		failure = err
	}
	c.Bytes, c.Dropped = size, int(rec.dropped.Load()+mixed)
	c.Status = StatusDone
	if failure != nil {
		slog.Error("capturas: gravar arquivo", "id", c.ID, "erro", failure)
		c.Status, c.Error = StatusFailed, "erro ao gravar o arquivo"
	}
	r.finish(c, true)
}

// finish grava o fim da captura no banco e avisa a interface. Com release,
// libera a vez da próxima captura antes do aviso; Start, que já segura r.mu
// e ainda não marcou a captura como ativa, passa false.
func (r *Recorder) finish(c Capture, release bool) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	c.FinishedAt = &now
	ctx, cancel := context.WithTimeout(context.Background(), finishTimeout)
	defer cancel()
	if err := r.st.FinishCapture(ctx, c); err != nil {
		slog.Error("capturas: gravar o fim no banco", "id", c.ID, "erro", err)
	}
	if release {
		r.mu.Lock()
		r.active = 0
		r.mu.Unlock()
	}
	r.pub.Publish(ws.Event{Type: EventFinished, Data: c})
}

// List devolve as capturas, das mais novas para as mais antigas.
func (r *Recorder) List(ctx context.Context) ([]Capture, error) { return r.st.ListCaptures(ctx) }

// Open abre o arquivo de uma captura que já terminou. Quem chama fecha o arquivo.
func (r *Recorder) Open(ctx context.Context, id int64) (*os.File, Capture, error) {
	c, err := r.st.GetCapture(ctx, id)
	if err != nil {
		return nil, c, err
	}
	if c.Status == StatusRecording {
		return nil, c, ErrAindaGravando
	}
	path, ok := r.path(c)
	if !ok {
		return nil, c, ErrArquivoIndisponivel
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, c, ErrArquivoIndisponivel
	}
	return f, c, err
}

// Packets devolve os resumos da página page (a partir de 0) da captura, com
// as portas dos alvos de agora classificando os protocolos.
func (r *Recorder) Packets(ctx context.Context, id int64, page int) ([]sniffer.PacketSummary, Capture, error) {
	f, c, err := r.Open(ctx, id)
	if err != nil {
		return nil, c, err
	}
	defer func() { _ = f.Close() }()
	ts, err := r.targets(ctx)
	if err != nil {
		return nil, c, err
	}
	roles, db := sniffer.RolesFor(ts), sniffer.DBPorts(ts)
	list := []sniffer.PacketSummary{}
	pr, err := pcapgo.NewReader(f)
	if err != nil {
		// Um arquivo cortado antes do cabeçalho não tem pacotes para mostrar.
		return list, c, nil
	}
	for range page * PageSize {
		if _, _, err := pr.ReadPacketData(); err != nil {
			return list, c, nil
		}
	}
	for len(list) < PageSize {
		data, ci, err := pr.ReadPacketData()
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				slog.Warn("capturas: ler arquivo", "id", id, "erro", err)
			}
			break
		}
		p := gopacket.NewPacket(data, pr.LinkType(), gopacket.DecodeOptions{Lazy: true, NoCopy: true})
		p.Metadata().CaptureInfo = ci
		list = append(list, sniffer.Summarize(p, roles, db))
	}
	return list, c, nil
}

// Delete apaga o arquivo e a linha de uma captura que já terminou.
func (r *Recorder) Delete(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == id {
		return ErrAindaGravando
	}
	c, err := r.st.GetCapture(ctx, id)
	if err != nil {
		return err
	}
	if err := r.removeFile(c); err != nil {
		return err
	}
	return r.st.DeleteCapture(ctx, id)
}

// Run marca as capturas interrompidas no último encerramento e, a cada
// minuto até ctx acabar, apaga as mais antigas que a retenção.
func (r *Recorder) Run(ctx context.Context) {
	if err := r.st.FailInterruptedCaptures(ctx); err != nil && ctx.Err() == nil {
		slog.Error("capturas: marcar capturas interrompidas", "erro", err)
	}
	t := time.NewTicker(pruneEvery)
	defer t.Stop()
	for {
		r.Prune(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Prune apaga os arquivos e as linhas das capturas iniciadas antes de
// now menos a retenção.
func (r *Recorder) Prune(ctx context.Context, now time.Time) {
	old, err := r.st.CapturesBefore(ctx, now.Add(-r.retention))
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("capturas: listar capturas antigas", "erro", err)
		}
		return
	}
	for _, c := range old {
		if err := r.Delete(ctx, c.ID); err != nil && !errors.Is(err, ErrNotFound) && ctx.Err() == nil {
			slog.Error("capturas: apagar captura antiga", "id", c.ID, "erro", err)
		}
	}
}

// path devolve o caminho do arquivo; um nome com diretório nunca sai do banco
// pelo Omni, mas é recusado para nada fora do diretório ser lido ou apagado.
func (r *Recorder) path(c Capture) (string, bool) {
	if c.File == "" || filepath.Base(c.File) != c.File {
		return "", false
	}
	return filepath.Join(r.dir, c.File), true
}

func (r *Recorder) removeFile(c Capture) error {
	path, ok := r.path(c)
	if !ok {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("apagar o arquivo da captura: %w", err)
	}
	return nil
}

// dirSize soma o tamanho dos arquivos do diretório, sem descer em subdiretórios.
func dirSize(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return 0, err
		}
		total += info.Size()
	}
	return total, nil
}
