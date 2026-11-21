// Package findings define o achado de segurança normalizado do Arsenal, igual
// para todas as ferramentas.
package findings

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
)

// Severity é a severidade normalizada de um achado.
type Severity string

const (
	Info     Severity = "info"
	Low      Severity = "baixa"
	Medium   Severity = "media"
	High     Severity = "alta"
	Critical Severity = "critica"
)

// Severities lista as severidades da mais grave para a menos grave.
var Severities = []Severity{Critical, High, Medium, Low, Info}

// Rank ordena as severidades: critica é 4, info é 0; desconhecida é -1.
func (s Severity) Rank() int {
	for i, v := range Severities {
		if v == s {
			return len(Severities) - 1 - i
		}
	}
	return -1
}

// FromTool traduz a severidade em inglês usada por Nuclei e testssl.
func FromTool(s string) Severity {
	switch s {
	case "critical", "CRITICAL":
		return Critical
	case "high", "HIGH":
		return High
	case "medium", "MEDIUM":
		return Medium
	case "low", "LOW":
		return Low
	default:
		return Info
	}
}

// Finding é um achado de uma ferramenta. Fingerprint identifica o mesmo
// achado entre varreduras, para a comparação.
type Finding struct {
	Tool        string   `json:"ferramenta"`
	Title       string   `json:"titulo"`
	Evidence    string   `json:"evidencia"`
	Severity    Severity `json:"severidade"`
	Port        int      `json:"porta"`
	Fingerprint string   `json:"assinatura"`
}

// New monta um achado e calcula o Fingerprint.
func New(tool, title, evidence string, sev Severity, port int) Finding {
	return Finding{Tool: tool, Title: title, Evidence: evidence, Severity: sev, Port: port, Fingerprint: Fingerprint(tool, title, port)}
}

// Fingerprint é o sha256 de "ferramenta|título|porta", em hexadecimal.
func Fingerprint(tool, title string, port int) string {
	sum := sha256.Sum256([]byte(tool + "|" + title + "|" + strconv.Itoa(port)))
	return hex.EncodeToString(sum[:])
}

// Sort ordena da severidade mais grave para a menos grave, depois por porta e título.
func Sort(list []Finding) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Severity.Rank() != b.Severity.Rank() {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		return a.Title < b.Title
	})
}
