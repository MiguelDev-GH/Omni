// Package capture grava capturas .pcap curtas, sob demanda, a partir do
// tráfego que o sniffer já captura nas portas dos alvos.
package capture

import (
	"errors"
	"time"
)

// Estados de uma captura.
const (
	StatusRecording = "gravando"
	StatusDone      = "concluida"
	StatusFailed    = "falhou"
)

// Capture é uma gravação .pcap. File é o nome do arquivo dentro do diretório
// das capturas e nunca sai na API.
type Capture struct {
	ID         int64      `json:"id"`
	File       string     `json:"-"`
	StartedAt  time.Time  `json:"inicio"`
	FinishedAt *time.Time `json:"fim,omitempty"`
	// Duration é a duração pedida, em segundos; a gravação para antes se
	// chegar ao limite de tamanho.
	Duration int `json:"duracao_s"`
	// IP, Port e Protocol são o filtro do Inspetor usado na gravação; vazios
	// (e porta 0) gravam todo o tráfego das portas dos alvos.
	IP       string `json:"ip"`
	Port     int    `json:"porta"`
	Protocol string `json:"protocolo"`
	Packets  int    `json:"pacotes"`
	Bytes    int64  `json:"bytes"`
	Dropped  int    `json:"descartados"`
	Status   string `json:"status"`
	Error    string `json:"erro,omitempty"`
}

var (
	ErrNotFound            = errors.New("captura não encontrada")
	ErrDuracaoInvalida     = errors.New("a duração precisa ficar entre 10 s e 300 s")
	ErrFiltroInvalido      = errors.New("filtro inválido: use um endereço IP, uma porta de 0 a 65535 e um protocolo da lista")
	ErrEmAndamento         = errors.New("já há uma captura gravando; espere ela terminar")
	ErrDiretorioCheio      = errors.New("o diretório das capturas passou de 1 GiB; apague capturas antigas")
	ErrSnifferInativo      = errors.New("o sniffer não está capturando agora")
	ErrAindaGravando       = errors.New("a captura ainda está gravando")
	ErrArquivoIndisponivel = errors.New("o arquivo da captura não existe mais")
)
