package l7

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBStatsWindow(t *testing.T) {
	s := NewDBStats()
	s.Flush(t0)
	for i := 1; i <= 20; i++ {
		s.Add(DBQuery{Query: "SELECT * FROM u WHERE id = ?", Kind: "SELECT", Latency: time.Duration(i) * time.Millisecond})
	}
	s.Add(DBQuery{Query: "INSERT INTO t VALUES (?)", Kind: "INSERT", Latency: time.Millisecond, Error: true})
	w := s.Flush(t0.Add(10 * time.Second))
	assert.Equal(t, uint64(21), w.Queries)
	assert.Equal(t, uint64(1), w.Errors)
	assert.Equal(t, map[string]uint64{"SELECT": 20, "INSERT": 1}, w.ByKind)
	require.Len(t, w.Top, 2)
	assert.Equal(t, "SELECT", w.Top[0].Kind)
	assert.Equal(t, 10.0, w.Top[0].P50MS)
	assert.Equal(t, 19.0, w.Top[0].P95MS)
	assert.Equal(t, uint64(1), w.Top[1].Errors)
	assert.Empty(t, s.Flush(t0.Add(20*time.Second)).Top)
}

func TestDBStatsCountsTxFailures(t *testing.T) {
	s := NewDBStats()
	s.Flush(t0)
	s.AddTxFailure()
	s.AddTxFailure()
	w := s.Flush(t0.Add(10 * time.Second))
	assert.Equal(t, uint64(2), w.TxFailed)
	assert.Equal(t, uint64(0), s.Flush(t0.Add(20*time.Second)).TxFailed)
}
