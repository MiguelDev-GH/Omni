package l7

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPStatsPercentiles(t *testing.T) {
	s := NewHTTPStats()
	s.Flush(t0)
	for i := 1; i <= 100; i++ {
		s.Add(HTTPExchange{Method: "GET", Route: "/a", Status: 200, Latency: time.Duration(i) * time.Millisecond})
	}
	s.Add(HTTPExchange{Method: "POST", Route: "/b", Status: 503, Latency: 7 * time.Millisecond})
	s.Add(HTTPExchange{Method: "GET", Route: "/a", Status: 200, Latency: time.Hour, Internal: true})

	w := s.Flush(t0.Add(10 * time.Second))
	assert.Equal(t, t0, w.Start)
	assert.Equal(t, uint64(101), w.Requests)
	assert.Equal(t, map[string]uint64{"2xx": 100, "5xx": 1}, w.ByStatus)
	require.Len(t, w.Routes, 2)
	a := w.Routes[0]
	assert.Equal(t, [2]string{"GET", "/a"}, [2]string{a.Method, a.Route})
	assert.Equal(t, uint64(100), a.Requests)
	assert.Equal(t, 50.0, a.P50MS)
	assert.Equal(t, 95.0, a.P95MS)
	assert.Equal(t, uint64(1), w.Routes[1].Errors)
	assert.InDelta(t, 95.0, a.Hist.Quantile(0.95), 95*0.25)

	next := s.Flush(t0.Add(20 * time.Second))
	assert.Equal(t, uint64(0), next.Requests)
	assert.Empty(t, next.Routes)
	assert.Equal(t, t0.Add(10*time.Second), next.Start)
}

func TestHTTPStatsCapsRoutes(t *testing.T) {
	s := NewHTTPStats()
	for i := range maxGroups + 20 {
		s.Add(HTTPExchange{Method: "GET", Route: fmt.Sprintf("/r%03d", i), Status: 200, Latency: time.Millisecond})
	}
	w := s.Flush(t0)
	require.Len(t, w.Routes, maxGroups+1)
	last := w.Routes[maxGroups]
	assert.Equal(t, Others, last.Route)
	assert.Equal(t, uint64(20), last.Requests)
}

func TestHistogramMergeAndQuantile(t *testing.T) {
	h := newHistogram()
	for range 90 {
		h.observe(1)
	}
	o := newHistogram()
	for range 10 {
		o.observe(1000)
	}
	m := h.Merge(o)
	assert.InDelta(t, 1, m.Quantile(0.5), 0.25)
	assert.InDelta(t, 1000, m.Quantile(0.95), 250)
	assert.Equal(t, 0.0, newHistogram().Quantile(0.5))
	assert.Equal(t, 3.0, Percentile([]float64{5, 1, 3}, 0.5))
}
