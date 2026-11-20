package ws

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTicketIsSingleUse(t *testing.T) {
	tk := NewTickets(30 * time.Second)
	ticket, err := tk.Issue()
	require.NoError(t, err)
	assert.Len(t, ticket, 64)
	_, err = hex.DecodeString(ticket)
	require.NoError(t, err)

	assert.True(t, tk.Consume(ticket))
	assert.False(t, tk.Consume(ticket))
}

func TestTicketExpires(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	tk := NewTickets(30 * time.Second)
	tk.now = func() time.Time { return now }

	ticket, err := tk.Issue()
	require.NoError(t, err)

	now = now.Add(31 * time.Second)
	assert.False(t, tk.Consume(ticket))
}

func TestUnknownTicket(t *testing.T) {
	assert.False(t, NewTickets(30*time.Second).Consume("abc"))
}
