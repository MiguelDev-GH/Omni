package arsenal

import (
	"io"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// Tool descreve uma ferramenta do Arsenal.
type Tool struct {
	Name        string `json:"nome"`
	Description string `json:"descricao"`
	Image       string `json:"imagem"`
	// Args monta os argumentos para o alvo; recusa alvo inválido ou fora do escopo.
	Args func(target.Target) ([]string, error) `json:"-"`
	// Parse lê a saída padrão da ferramenta.
	Parse func(io.Reader) ([]findings.Finding, error) `json:"-"`
	// Volumes são volumes nomeados do Docker (nome → caminho no container),
	// como o cache de templates do Nuclei.
	Volumes map[string]string `json:"-"`
	// AnyExit aceita código de saída diferente de zero (o testssl usa o
	// código para contar problemas).
	AnyExit bool `json:"-"`
}

// Tools são as ferramentas disponíveis, na ordem exibida na interface.
var Tools = []Tool{
	{
		Name:        "nmap",
		Description: "Portas abertas e versões dos serviços",
		Image:       NmapImage,
		Args:        NmapArgs,
		Parse:       ParseNmapXML,
	},
}

// Lookup devolve a ferramenta pelo nome.
func Lookup(name string) (Tool, bool) {
	for _, t := range Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
