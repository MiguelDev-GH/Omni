package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait    = 10 * time.Second
	pongWait     = 60 * time.Second
	pingInterval = 30 * time.Second
	readLimit    = 512
)

// A origem já foi conferida em Handler, antes de gastar o ticket.
var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

// Handler serve GET /ws?ticket=..., conectando o cliente ao hub.
func Handler(h *Hub, t *Tickets, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "método não permitido")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !slices.Contains(allowedOrigins, origin) {
			writeError(w, http.StatusForbidden, "origem não permitida")
			return
		}
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" || !t.Consume(ticket) {
			writeError(w, http.StatusUnauthorized, "ticket inválido")
			return
		}
		c, ok := h.subscribe()
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "servidor desligando")
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade já respondeu ao cliente com o erro.
			h.unsubscribe(c)
			return
		}

		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			readLoop(conn, h, c)
		}()
		writeLoop(conn, c)
		h.unsubscribe(c)
		_ = conn.Close()
		<-readDone
	})
}

// writeLoop envia as mensagens do cliente e pings; sai em erro de escrita ou
// quando o cliente é encerrado, mandando um close frame.
func writeLoop(conn *websocket.Conn, c *client) {
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case msg := <-c.send:
			if err := write(conn, websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			if err := write(conn, websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.done:
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
			_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(writeWait))
			return
		}
	}
}

func write(conn *websocket.Conn, kind int, msg []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return conn.WriteMessage(kind, msg)
}

// readLoop descarta o que o cliente envia e mantém o deadline renovado pelos pongs.
func readLoop(conn *websocket.Conn, h *Hub, c *client) {
	defer h.unsubscribe(c)
	conn.SetReadLimit(readLimit)
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// TicketHandler emite um ticket de uso único para abrir o WebSocket.
func TicketHandler(t *Tickets) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ticket, err := t.Issue()
		if err != nil {
			slog.Error("gerar ticket", "erro", err)
			writeError(w, http.StatusInternalServerError, "erro ao gerar ticket")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ticket": ticket})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("escrever resposta", "erro", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"erro": msg})
}
