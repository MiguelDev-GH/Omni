package l7

import (
	"strconv"
	"time"
)

// SMTPWindow resume as transações SMTP de um intervalo, normalmente 10 s.
type SMTPWindow struct {
	Start     time.Time         `json:"inicio"`
	Attempts  uint64            `json:"tentativas"` // transações iniciadas com MAIL FROM
	Delivered uint64            `json:"entregues"`
	Rejected  uint64            `json:"rejeitados"`
	Aborted   uint64            `json:"abortados"`
	ByCode    map[string]uint64 `json:"codigos"` // só as respostas 4xx e 5xx
	// P50MS e P95MS são a latência de envio das mensagens entregues.
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
}

// SMTPStats agrega os SMTPExchange da janela atual. Não é seguro para uso
// concorrente.
type SMTPStats struct {
	start                                 time.Time
	attempts, delivered, rejected, aborts uint64
	byCode                                map[string]uint64
	sent                                  group
}

func NewSMTPStats() *SMTPStats {
	s := &SMTPStats{}
	s.reset(time.Time{})
	return s
}

func (s *SMTPStats) reset(start time.Time) {
	s.start = start
	s.attempts, s.delivered, s.rejected, s.aborts = 0, 0, 0, 0
	s.byCode = map[string]uint64{}
	s.sent = group{}
}

// Add conta ex na janela.
func (s *SMTPStats) Add(ex SMTPExchange) {
	s.attempts++
	switch ex.Outcome {
	case OutcomeDelivered:
		s.delivered++
		s.sent.observe(ex.Latency, false)
	case OutcomeRejected:
		s.rejected++
	default:
		s.aborts++
	}
	if ex.Code >= 400 {
		s.byCode[strconv.Itoa(ex.Code)]++
	}
}

// Flush devolve a janela iniciada no Flush anterior e começa outra em now.
// A primeira janela começa em now.
func (s *SMTPStats) Flush(now time.Time) SMTPWindow {
	start := s.start
	if start.IsZero() {
		start = now
	}
	w := SMTPWindow{Start: start.UTC(), Attempts: s.attempts, Delivered: s.delivered, Rejected: s.rejected,
		Aborted: s.aborts, ByCode: s.byCode}
	w.P50MS, w.P95MS = s.sent.quantiles()
	s.reset(now)
	return w
}
