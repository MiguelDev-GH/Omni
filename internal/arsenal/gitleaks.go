package arsenal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MiguelDev-GH/omni/internal/findings"
	"github.com/MiguelDev-GH/omni/internal/target"
)

// GitleaksImage é a imagem oficial do Gitleaks, com versão fixa.
const GitleaksImage = "zricethezav/gitleaks:v8.30.0"

// RepoMount é onde o código-fonte do alvo aparece no container, sempre
// montado como somente leitura.
const RepoMount = "/repo"

// GitleaksArgs devolve os argumentos do Gitleaks para o código-fonte do
// alvo montado em RepoMount. Com histórico do git, ele varre todos os
// commits (e o relatório traz commit, autor e data); sem, varre só os
// arquivos. --redact já tira o valor da saída, e o parser ainda descarta os
// campos do segredo; --exit-code 0 porque achar vazamento não é falha.
// "--report-path -" escreve o JSON na saída padrão (/dev/stdout não
// funciona nessa versão: o relatório sai vazio).
func GitleaksArgs(t target.Target, o Options) ([]string, error) {
	if err := checkTarget(t); err != nil {
		return nil, err
	}
	if t.Repo == "" {
		return nil, &target.ValidationError{Field: "repositorio", Message: "o Gitleaks precisa do código-fonte do alvo"}
	}
	if err := noOptions("gitleaks", o); err != nil {
		return nil, err
	}
	mode := "dir"
	if _, err := os.Stat(filepath.Join(t.Repo, ".git")); err == nil {
		mode = "git"
	}
	return []string{mode, RepoMount, "--report-format", "json", "--report-path", "-",
		"--redact", "--exit-code", "0", "--no-banner", "--log-level", "warn"}, nil
}

// GitleaksReport são os vazamentos encontrados, sem o valor do segredo.
type GitleaksReport struct {
	Leaks []Leak `json:"vazamentos"`
}

type Leak struct {
	Rule        string `json:"regra"`
	Description string `json:"descricao"`
	// File é relativo à raiz do código-fonte.
	File string `json:"arquivo"`
	Line int    `json:"linha"`
	// Commit, Author e Date ficam vazios quando o código-fonte não tem
	// histórico do git.
	Commit  string  `json:"commit"`
	Author  string  `json:"autor"`
	Date    string  `json:"data"`
	Entropy float64 `json:"entropia"`
}

// gitleaksLeak é o que se lê de cada vazamento. De propósito, não tem os
// campos Secret, Match, Email nem Message: o valor do segredo e o trecho
// casado nunca chegam à memória do Omni, nem ao banco ou à interface.
type gitleaksLeak struct {
	RuleID      string  `json:"RuleID"`
	Description string  `json:"Description"`
	StartLine   int     `json:"StartLine"`
	File        string  `json:"File"`
	Commit      string  `json:"Commit"`
	Author      string  `json:"Author"`
	Date        string  `json:"Date"`
	Entropy     float64 `json:"Entropy"`
}

// gitleaksLeaks lê o JSON do Gitleaks, ordenado por arquivo e linha.
func gitleaksLeaks(out []byte) ([]Leak, error) {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil, errors.New("o gitleaks não devolveu relatório")
	}
	var raw []gitleaksLeak
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("ler JSON do gitleaks: %w", err)
	}
	leaks := make([]Leak, 0, len(raw))
	for _, l := range raw {
		// Sem histórico do git, o caminho vem com o ponto de montagem.
		file := strings.TrimPrefix(strings.TrimPrefix(l.File, RepoMount), "/")
		leaks = append(leaks, Leak{
			Rule: l.RuleID, Description: l.Description, File: file, Line: l.StartLine,
			Commit: l.Commit, Author: l.Author, Date: l.Date, Entropy: l.Entropy,
		})
	}
	sort.SliceStable(leaks, func(i, j int) bool {
		if leaks[i].File != leaks[j].File {
			return leaks[i].File < leaks[j].File
		}
		return leaks[i].Line < leaks[j].Line
	})
	return leaks, nil
}

// ParseGitleaksJSON devolve um achado alta por vazamento; a evidência só
// tem o commit e a data, nunca o valor.
func ParseGitleaksJSON(r io.Reader) ([]findings.Finding, error) {
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	leaks, err := gitleaksLeaks(out)
	if err != nil {
		return nil, err
	}
	list := make([]findings.Finding, 0, len(leaks))
	for _, l := range leaks {
		title := l.Rule + " em " + l.File + ":" + strconv.Itoa(l.Line)
		evidence := "fora do histórico do git"
		if l.Commit != "" {
			evidence = "commit " + l.Commit
			if l.Date != "" {
				evidence += "; data " + l.Date
			}
		}
		list = append(list, findings.New("gitleaks", truncate(title, maxEvidence), evidence, findings.High, 0))
	}
	return list, nil
}

// GitleaksReportFrom monta o GitleaksReport.
func GitleaksReportFrom(out []byte) (json.RawMessage, error) {
	leaks, err := gitleaksLeaks(out)
	if err != nil {
		return nil, err
	}
	return json.Marshal(GitleaksReport{Leaks: leaks})
}
