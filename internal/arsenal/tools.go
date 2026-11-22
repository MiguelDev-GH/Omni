package arsenal

import (
	"encoding/json"
	"errors"
	"io"
	"sort"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// ErrOpcaoInvalida indica uma opção de ferramenta desconhecida ou com valor
// fora do permitido.
var ErrOpcaoInvalida = errors.New("opção inválida")

// Options são as opções de uma ferramenta vindas da interface (chave →
// valor). Cada ferramenta valida as suas contra uma lista fechada.
type Options map[string]string

// OptionError é uma opção recusada; Field aponta o campo da API.
type OptionError struct {
	Tool string
	// Key é a opção recusada; vazia quando a ferramenta inteira não aceita
	// as opções enviadas.
	Key string
}

func (e *OptionError) Error() string {
	if e.Key == "" {
		return ErrOpcaoInvalida.Error() + ": " + e.Tool
	}
	return ErrOpcaoInvalida.Error() + ": " + e.Key
}

func (e *OptionError) Unwrap() error { return ErrOpcaoInvalida }

// Field devolve o campo da API, como "opcoes.nmap.portas".
func (e *OptionError) Field() string {
	if e.Key == "" {
		return "opcoes." + e.Tool
	}
	return "opcoes." + e.Tool + "." + e.Key
}

// noOptions recusa qualquer opção, para as ferramentas que não têm nenhuma.
func noOptions(tool string, o Options) error {
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	return &OptionError{Tool: tool, Key: keys[0]}
}

// Tool descreve uma ferramenta do Arsenal.
type Tool struct {
	Name        string `json:"nome"`
	Description string `json:"descricao"`
	Image       string `json:"imagem"`
	// Requires é o dado opcional do alvo de que a ferramenta precisa:
	// "", "dominio" ou "repositorio".
	Requires string `json:"requer,omitempty"`
	// Args monta os argumentos para o alvo e as opções; recusa alvo
	// inválido, fora do escopo ou opção inválida (*OptionError).
	Args func(target.Target, Options) ([]string, error) `json:"-"`
	// Parse lê a saída padrão da ferramenta.
	Parse func(io.Reader) ([]findings.Finding, error) `json:"-"`
	// Report monta, a partir da saída padrão, o relatório estruturado que
	// alimenta a página da ferramenta; nil quando ela não tem relatório.
	Report func(stdout []byte) (json.RawMessage, error) `json:"-"`
	// Files devolve os arquivos copiados para o container antes de ele
	// iniciar (caminho absoluto → conteúdo), como a lista de palavras do
	// ffuf; nil quando a ferramenta não precisa de nenhum.
	Files func(Options) (map[string][]byte, error) `json:"-"`
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
		Report:      NmapReportFrom,
	},
	{
		Name:        "nuclei",
		Description: "Vulnerabilidades conhecidas (CVEs), exposições e más configurações",
		Image:       NucleiImage,
		Args:        NucleiArgs,
		Parse:       ParseNucleiJSONL,
		Report:      NucleiReportFrom,
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
		Report:      TestsslReportFrom,
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
