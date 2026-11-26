// Package dbsize mede o tamanho dos bancos do PostgreSQL por uma conexão
// opcional, só de leitura, e estima o crescimento em GB por dia.
package dbsize

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// DefaultInterval é o intervalo padrão entre medições.
const DefaultInterval = 5 * time.Minute

const (
	connectTimeout = 5 * time.Second
	queryTimeout   = 10 * time.Second
	pruneEvery     = time.Hour
	// minGrowthSpan é o intervalo mínimo entre a primeira e a última
	// amostra para estimar o crescimento.
	minGrowthSpan = time.Hour
	bytesPerGB    = 1e9
)

// Sample é o tamanho de um banco num instante.
type Sample struct {
	Name  string    `json:"banco"`
	Bytes int64     `json:"bytes"`
	At    time.Time `json:"hora"`
}

// Querier lê o tamanho de todos os bancos.
type Querier func(ctx context.Context) ([]Sample, error)

// Sink guarda as medições para o histórico.
type Sink interface {
	AddDBSizes(ctx context.Context, list []Sample) error
	// PruneDBSizes apaga as medições anteriores a before.
	PruneDBSizes(ctx context.Context, before time.Time) error
}

// PGQuerier mede os bancos com uma conexão nova a cada chamada, fechada no
// fim. As mensagens de erro nunca trazem a senha do DSN.
func PGQuerier(dsn string) Querier {
	return func(ctx context.Context) ([]Sample, error) {
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			// A mensagem do pgx pode repetir o DSN; fica só a causa genérica.
			return nil, errors.New("ler tamanho do PostgreSQL: OMNI_PG_DSN inválido")
		}
		if cfg.ConnectTimeout == 0 || cfg.ConnectTimeout > connectTimeout {
			cfg.ConnectTimeout = connectTimeout
		}
		ctx, cancel := context.WithTimeout(ctx, queryTimeout)
		defer cancel()
		conn, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			return nil, failure(err, cfg.Password)
		}
		defer func() { _ = conn.Close(context.Background()) }()
		rows, err := conn.Query(ctx, `SELECT datname, pg_database_size(datname) FROM pg_database WHERE NOT datistemplate ORDER BY datname`)
		if err != nil {
			return nil, failure(err, cfg.Password)
		}
		defer rows.Close()
		now := time.Now().UTC()
		var list []Sample
		for rows.Next() {
			s := Sample{At: now}
			if err := rows.Scan(&s.Name, &s.Bytes); err != nil {
				return nil, failure(err, cfg.Password)
			}
			list = append(list, s)
		}
		if err := rows.Err(); err != nil {
			return nil, failure(err, cfg.Password)
		}
		return list, nil
	}
}

func failure(err error, password string) error {
	msg := err.Error()
	if password != "" {
		msg = strings.ReplaceAll(msg, password, "xxxxx")
	}
	return errors.New("ler tamanho do PostgreSQL: " + msg)
}

// Run mede os bancos a cada interval até ctx ser cancelado e grava no sink.
// Falhas são registradas e o ciclo segue. Uma vez por hora, apaga as
// medições mais antigas que retention.
func Run(ctx context.Context, q Querier, sink Sink, interval, retention time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastPrune time.Time
	for {
		list, err := q(ctx)
		switch {
		case err == nil:
			if err := sink.AddDBSizes(ctx, list); err != nil && ctx.Err() == nil {
				slog.Error("banco: gravar tamanho", "erro", err)
			}
			if now := time.Now(); now.Sub(lastPrune) >= pruneEvery {
				lastPrune = now
				if err := sink.PruneDBSizes(ctx, now.Add(-retention)); err != nil && ctx.Err() == nil {
					slog.Error("banco: apagar tamanhos antigos", "erro", err)
				}
			}
		case ctx.Err() == nil:
			slog.Error("banco: medir tamanho", "erro", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Growth devolve o crescimento em GB/dia (GB = 10⁹ bytes) pela regressão
// linear dos bytes no tempo. Devolve ok=false com menos de 2 amostras ou
// quando elas cobrem menos de 1 h.
func Growth(samples []Sample) (gbPerDay float64, ok bool) {
	if len(samples) < 2 {
		return 0, false
	}
	first, last := samples[0].At, samples[0].At
	for _, s := range samples {
		if s.At.Before(first) {
			first = s.At
		}
		if s.At.After(last) {
			last = s.At
		}
	}
	if last.Sub(first) < minGrowthSpan {
		return 0, false
	}
	var mx, my float64
	for _, s := range samples {
		mx += s.At.Sub(first).Seconds()
		my += float64(s.Bytes)
	}
	n := float64(len(samples))
	mx, my = mx/n, my/n
	var sxy, sxx float64
	for _, s := range samples {
		dx := s.At.Sub(first).Seconds() - mx
		sxy += dx * (float64(s.Bytes) - my)
		sxx += dx * dx
	}
	bytesPerSecond := sxy / sxx
	return bytesPerSecond * 86400 / bytesPerGB, true
}
