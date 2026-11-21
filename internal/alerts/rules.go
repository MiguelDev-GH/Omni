// Package alerts avalia as métricas do Omni contra limites, classifica cada
// recurso em verde, amarelo ou vermelho e avisa quando o nível muda.
package alerts

import "time"

// Level é o nível de um alerta, na escala pedida pelo trabalho.
type Level string

const (
	// Green é o estado normal.
	Green Level = "verde"
	// Yellow indica degradação que exige observação.
	Yellow Level = "amarelo"
	// Red indica serviço degradado ou indisponível.
	Red Level = "vermelho"
)

func (l Level) rank() int {
	switch l {
	case Red:
		return 2
	case Yellow:
		return 1
	default:
		return 0
	}
}

// Serviços monitorados, usados para agrupar os alertas na interface.
const (
	ServiceWeb    = "web"
	ServiceDB     = "banco"
	ServiceDNS    = "dns"
	ServiceSMTP   = "smtp"
	ServiceSystem = "sistema"
)

// Identificadores das regras padrão.
const (
	RuleAvailability = "disponibilidade"
	RuleProbeLatency = "latencia_verificacao"
	RuleHTTP5xx      = "http_erros_5xx"
	RuleHTTPP95      = "http_p95"
	RuleDBErrors     = "banco_erros"
	RuleDBP95        = "banco_p95"
	RuleDNSLatency   = "dns_latencia"
	RuleSMTPLatency  = "smtp_latencia"
	RuleProbeErrors  = "taxa_falhas_sonda"
	RuleSMTPNoTLS    = "smtp_sem_starttls"
	RuleCPU          = "sistema_cpu"
	RuleMemory       = "sistema_memoria"
	RuleDisk         = "sistema_disco"
)

// Rule define quando um valor vira amarelo ou vermelho. Um valor maior ou
// igual a Warn é amarelo; maior ou igual a Crit, vermelho. Uma mudança de
// nível só vale depois que a condição se mantém por For, o que evita
// alertas por um pico isolado.
type Rule struct {
	ID      string        `json:"id"`
	Service string        `json:"servico"`
	Name    string        `json:"nome"`
	Unit    string        `json:"unidade"`
	Warn    float64       `json:"limite_amarelo"`
	Crit    float64       `json:"limite_vermelho"`
	For     time.Duration `json:"-"`
}

// LevelFor classifica um valor isolado, sem levar For em conta.
func (r Rule) LevelFor(v float64) Level {
	switch {
	case v >= r.Crit:
		return Red
	case v >= r.Warn:
		return Yellow
	default:
		return Green
	}
}

// threshold devolve o limite que levou ao nível l.
func (r Rule) threshold(l Level) float64 {
	switch l {
	case Red:
		return r.Crit
	case Yellow:
		return r.Warn
	default:
		return 0
	}
}

// Valores da regra de disponibilidade: o Poller traduz o resultado de cada
// verificação para um destes números.
const (
	AvailabilityUp       = 0 // respondeu normalmente
	AvailabilityDegraded = 1 // respondeu com erro: HTTP 5xx, RCODE no DNS, recusa no SMTP
	AvailabilityDown     = 2 // não respondeu
)

// DefaultRules são os limites usados quando nada é configurado. Foram
// calibrados para a loja cobaia ficar verde com a carga padrão e mudar de
// nível quando alguém força lentidão, erros ou queda.
func DefaultRules() []Rule {
	return []Rule{
		{ID: RuleAvailability, Service: ServiceWeb, Name: "Disponibilidade", Unit: "",
			Warn: AvailabilityDegraded, Crit: AvailabilityDown},
		{ID: RuleProbeLatency, Service: ServiceWeb, Name: "Latência da verificação ativa", Unit: "ms",
			Warn: 1000, Crit: 3000, For: time.Minute},
		{ID: RuleHTTP5xx, Service: ServiceWeb, Name: "Taxa de erros 5xx", Unit: "%",
			Warn: 10, Crit: 25, For: 30 * time.Second},
		{ID: RuleHTTPP95, Service: ServiceWeb, Name: "Latência p95 das requisições", Unit: "ms",
			Warn: 1000, Crit: 2500, For: 30 * time.Second},
		{ID: RuleDBErrors, Service: ServiceDB, Name: "Taxa de erros nas consultas", Unit: "%",
			Warn: 10, Crit: 25, For: 30 * time.Second},
		{ID: RuleDBP95, Service: ServiceDB, Name: "Latência p95 das consultas", Unit: "ms",
			Warn: 500, Crit: 2000, For: 30 * time.Second},
		{ID: RuleDNSLatency, Service: ServiceDNS, Name: "Tempo de resolução DNS", Unit: "ms",
			Warn: 200, Crit: 1000, For: 30 * time.Second},
		{ID: RuleSMTPLatency, Service: ServiceSMTP, Name: "Latência da conversa SMTP", Unit: "ms",
			Warn: 2000, Crit: 5000, For: 30 * time.Second},
		{ID: RuleProbeErrors, Service: ServiceDNS, Name: "Taxa de falhas da sonda", Unit: "%",
			Warn: 20, Crit: 50},
		{ID: RuleSMTPNoTLS, Service: ServiceSMTP, Name: "SMTP sem STARTTLS", Unit: "",
			Warn: 1, Crit: 2},
		{ID: RuleCPU, Service: ServiceSystem, Name: "Uso de CPU", Unit: "%",
			Warn: 80, Crit: 95, For: time.Minute},
		{ID: RuleMemory, Service: ServiceSystem, Name: "Uso de memória", Unit: "%",
			Warn: 85, Crit: 95, For: time.Minute},
		{ID: RuleDisk, Service: ServiceSystem, Name: "Uso de disco", Unit: "%",
			Warn: 85, Crit: 95, For: time.Minute},
	}
}
