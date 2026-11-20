package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishDropsOldestWhenFull(t *testing.T) {
	h := NewHub(4)
	c, ok := h.subscribe()
	require.True(t, ok)

	for i := 1; i <= 10; i++ {
		h.Publish(Event{Type: "TESTE", Data: i})
	}

	var got []int
	for range 4 {
		select {
		case msg := <-c.send:
			var ev struct {
				Data int `json:"dados"`
			}
			require.NoError(t, json.Unmarshal(msg, &ev))
			got = append(got, ev.Data)
		default:
			t.Fatal("canal vazio antes do esperado")
		}
	}
	assert.Equal(t, []int{7, 8, 9, 10}, got)
}

func TestPublishWithoutClientsDoesNotBlock(t *testing.T) {
	h := NewHub(1)
	for i := range 1000 {
		h.Publish(Event{Type: "TESTE", Data: i})
	}
}

func TestCloseRejectsNewClients(t *testing.T) {
	h := NewHub(1)
	c, ok := h.subscribe()
	require.True(t, ok)

	h.Close()
	assert.NotPanics(t, h.Close)

	_, ok = h.subscribe()
	assert.False(t, ok)

	select {
	case <-c.done:
	default:
		t.Fatal("Close não sinalizou o cliente")
	}
	assert.NotPanics(t, func() { h.unsubscribe(c) })
	assert.NotPanics(t, func() { h.Publish(Event{Type: "TESTE"}) })
}
