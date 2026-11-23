package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePacket struct {
	Port int `json:"porta"`
}

func (p fakePacket) MatchesFilter(f InspectorFilter) bool { return f.Port == 0 || f.Port == p.Port }

type batch struct {
	Packets []fakePacket `json:"pacotes"`
	Dropped uint64       `json:"descartados"`
}

// readBatches lê os eventos já entregues a c e devolve os lotes PACOTES.
func readBatches(t *testing.T, c *client) (batches []batch, others int) {
	t.Helper()
	for {
		select {
		case msg := <-c.send:
			var ev struct {
				Type string          `json:"tipo"`
				Data json.RawMessage `json:"dados"`
			}
			require.NoError(t, json.Unmarshal(msg, &ev))
			if ev.Type != EventPackets {
				others++
				continue
			}
			var b batch
			require.NoError(t, json.Unmarshal(ev.Data, &b))
			batches = append(batches, b)
		default:
			return batches, others
		}
	}
}

func subscribed(t *testing.T, h *Hub, f *InspectorFilter) *client {
	t.Helper()
	c, ok := h.subscribe()
	require.True(t, ok)
	if f != nil {
		h.setInspect(c, f)
	}
	return c
}

func TestInspectorDropsWhenFull(t *testing.T) {
	h := NewHub(16)
	insp := NewInspector(h)
	c := subscribed(t, h, &InspectorFilter{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 100_000 {
			insp.Tap(fakePacket{Port: i})
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Tap bloqueou")
	}

	insp.flush()
	batches, _ := readBatches(t, c)
	require.Len(t, batches, 1)
	assert.Len(t, batches[0].Packets, inspectorMaxBatch)
	assert.Equal(t, uint64(100_000-inspectorMaxBatch), batches[0].Dropped)
	assert.Equal(t, inspectorQueue-1, batches[0].Packets[inspectorMaxBatch-1].Port, "fica com os mais recentes da fila")
}

func TestInspectorOnlyActiveClients(t *testing.T) {
	h := NewHub(16)
	insp := NewInspector(h)
	on := subscribed(t, h, &InspectorFilter{})
	off := subscribed(t, h, nil)
	stopped := subscribed(t, h, &InspectorFilter{})
	h.setInspect(stopped, nil)

	insp.Tap(fakePacket{Port: 80})
	insp.flush()
	h.Publish(Event{Type: "TESTE"})

	got, _ := readBatches(t, on)
	assert.Len(t, got, 1)
	for _, c := range []*client{off, stopped} {
		batches, others := readBatches(t, c)
		assert.Empty(t, batches)
		assert.Equal(t, 1, others, "continua recebendo os outros eventos")
	}
}

func TestInspectorFiltersPerClient(t *testing.T) {
	h := NewHub(16)
	insp := NewInspector(h)
	web := subscribed(t, h, &InspectorFilter{Port: 3000})
	all := subscribed(t, h, &InspectorFilter{})

	for _, p := range []int{3000, 5432, 3000} {
		insp.Tap(fakePacket{Port: p})
	}
	insp.flush()

	got, _ := readBatches(t, web)
	require.Len(t, got, 1)
	assert.Equal(t, []fakePacket{{3000}, {3000}}, got[0].Packets)
	got, _ = readBatches(t, all)
	require.Len(t, got, 1)
	assert.Len(t, got[0].Packets, 3)
}

func TestInspectorTapOffWithoutSubscribers(t *testing.T) {
	h := NewHub(16)
	insp := NewInspector(h)
	var states []bool
	insp.OnActive(func(on bool) { states = append(states, on) })

	insp.Tap(fakePacket{Port: 1})
	assert.Empty(t, insp.in, "sem assinantes, o tap não enfileira")

	a := subscribed(t, h, &InspectorFilter{})
	b := subscribed(t, h, &InspectorFilter{Port: 1})
	h.setInspect(a, &InspectorFilter{Port: 2}) // troca de filtro não muda o estado
	h.setInspect(a, nil)
	h.unsubscribe(b)

	assert.Equal(t, []bool{false, true, false}, states)
	assert.Zero(t, h.inspecting.Load())
}

func TestParseInspectorCommand(t *testing.T) {
	cmd, ok := parseInspectorCommand([]byte(`{"tipo":"INSPETOR","dados":{"ativo":true,"filtro":{"ip":"::ffff:10.0.0.1","porta":3000,"protocolo":"HTTP"}}}`))
	require.True(t, ok)
	assert.True(t, cmd.Active)
	assert.Equal(t, InspectorFilter{IP: "10.0.0.1", Port: 3000, Protocol: "HTTP"}, cmd.Filter)

	for _, msg := range []string{
		`{"tipo":"OUTRO","dados":{"ativo":true}}`,
		`não é json`,
		`{"tipo":"INSPETOR","dados":{"ativo":true,"filtro":{"ip":"evil.com"}}}`,
		`{"tipo":"INSPETOR","dados":{"ativo":true,"filtro":{"porta":70000}}}`,
	} {
		_, ok := parseInspectorCommand([]byte(msg))
		assert.False(t, ok, msg)
	}
}

func TestHandlerReadsInspectorCommand(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	h := NewHub(16)
	insp := NewInspector(h)
	srv := newWSServer(t, h, tk)
	conn := connect(t, srv, tk)
	waitClients(t, h, 1)

	require.NoError(t, conn.WriteMessage(1, []byte(`{"tipo":"INSPETOR","dados":{"ativo":true,"filtro":{"porta":3000}}}`)))
	require.Eventually(t, func() bool { return h.inspecting.Load() == 1 }, time.Second, 5*time.Millisecond)

	insp.Tap(fakePacket{Port: 22})
	insp.Tap(fakePacket{Port: 3000})
	insp.flush()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.JSONEq(t, `{"tipo":"PACOTES","dados":{"pacotes":[{"porta":3000}],"descartados":0}}`, string(msg))

	require.NoError(t, conn.WriteMessage(1, []byte(`{"tipo":"INSPETOR","dados":{"ativo":false}}`)))
	require.Eventually(t, func() bool { return h.inspecting.Load() == 0 }, time.Second, 5*time.Millisecond)
}
