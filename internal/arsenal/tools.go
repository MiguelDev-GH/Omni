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
	// Memory é o limite de memória do container em bytes; zero usa 512 MiB.
	Memory int64 `json:"-"`
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
	{
		Name:        "nuclei",
		Description: "Vulnerabilidades conhecidas (CVEs), exposições e más configurações",
		Image:       NucleiImage,
		Args:        NucleiArgs,
		Parse:       ParseNucleiJSONL,
		Volumes:     map[string]string{nucleiTemplates: "/root/nuclei-templates"},
		// Carregar os templates passa de 512 MiB: o pico medido é de cerca de 1 GiB.
		Memory: 1536 << 20,
	},
	{
		Name:        "testssl",
		Description: "Certificado e força da criptografia TLS",
		Image:       TestsslImage,
		Args:        TestsslArgs,
		Parse:       ParseTestsslJSON,
		AnyExit:     true,
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
