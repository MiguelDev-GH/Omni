// Package mailqueue mede o tamanho da fila de saída de um servidor de e-mail
// contando os arquivos dos diretórios da fila (como os do Postfix), sem
// abrir nenhum deles.
package mailqueue

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/MiguelDev-GH/omni/internal/ws"
)

// EventQueue é o tipo do evento publicado a cada contagem.
const EventQueue = "FILA_SMTP"

// DefaultInterval é o intervalo padrão entre contagens.
const DefaultInterval = 30 * time.Second

const pruneEvery = time.Hour

// Snapshot é uma contagem da fila. Err vem preenchido quando a contagem
// falhou (diretório ausente ou sem permissão); Messages fica 0.
type Snapshot struct {
	At       time.Time `json:"hora"`
	Messages int       `json:"mensagens"`
	Err      string    `json:"erro,omitempty"`
}

// Sink guarda as contagens para o histórico.
type Sink interface {
	AddQueueSnapshot(ctx context.Context, s Snapshot) error
	// PruneQueue apaga as contagens anteriores a before.
	PruneQueue(ctx context.Context, before time.Time) error
}

// Count conta os arquivos regulares sob os diretórios, sem abrir nenhum e
// sem seguir links simbólicos.
func Count(dirs []string) (int, error) {
	n := 0
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type().IsRegular() {
				n++
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return n, nil
}

// Run conta a fila a cada interval até ctx ser cancelado e publica cada
// contagem, inclusive as que falharam; só as bem-sucedidas vão para o sink.
// Uma vez por hora, apaga as contagens mais antigas que retention.
func Run(ctx context.Context, pub ws.Publisher, sink Sink, dirs []string, interval, retention time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastPrune time.Time
	for {
		snap := Snapshot{At: time.Now().UTC()}
		if n, err := Count(dirs); err != nil {
			snap.Err = err.Error()
			slog.Warn("fila do SMTP: contar mensagens", "erro", err)
		} else {
			snap.Messages = n
			if err := sink.AddQueueSnapshot(ctx, snap); err != nil && ctx.Err() == nil {
				slog.Error("fila do SMTP: gravar contagem", "erro", err)
			}
		}
		pub.Publish(ws.Event{Type: EventQueue, Data: snap})
		if snap.At.Sub(lastPrune) >= pruneEvery {
			lastPrune = snap.At
			if err := sink.PruneQueue(ctx, snap.At.Add(-retention)); err != nil && ctx.Err() == nil {
				slog.Error("fila do SMTP: apagar contagens antigas", "erro", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
