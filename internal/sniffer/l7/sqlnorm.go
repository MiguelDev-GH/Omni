package l7

import (
	"regexp"
	"strings"
)

const maxQueryLen = 1000

var (
	// listRe junta listas de marcadores entre parênteses, como "(?, ?, ?)".
	listRe = regexp.MustCompile(`\(\s*\?(\s*,\s*\?)*\s*\)`)
	// rowsRe junta linhas repetidas de VALUES, como "(?), (?)".
	rowsRe = regexp.MustCompile(`\(\?\)(\s*,\s*\(\?\))+`)
)

// NormalizeSQL troca literais de texto e números por "?", remove comentários,
// junta listas de marcadores e colapsa espaços, para que consultas que só
// diferem nos valores fiquem iguais e nenhum valor seja guardado.
func NormalizeSQL(q string) string {
	var b strings.Builder
	b.Grow(len(q))
	space := false
	emit := func(s string) {
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteString(s)
	}
	for i := 0; i < len(q); {
		c := q[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			space = true
			i++
		case c == '-' && i+1 < len(q) && q[i+1] == '-':
			for i < len(q) && q[i] != '\n' {
				i++
			}
			space = true
		case c == '/' && i+1 < len(q) && q[i+1] == '*':
			end := strings.Index(q[i+2:], "*/")
			if end < 0 {
				i = len(q)
			} else {
				i += end + 4
			}
			space = true
		case c == '\'' || ((c == 'E' || c == 'e') && i+1 < len(q) && q[i+1] == '\'' && !identBefore(q, i)):
			if c != '\'' {
				i++
			}
			i = skipString(q, i)
			emit("?")
		case c == '"':
			end := len(q)
			if j := strings.IndexByte(q[i+1:], '"'); j >= 0 {
				end = i + j + 2
			}
			emit(q[i:end]) // identificador entre aspas
			i = end
		case c == '$':
			if tag, ok := dollarTag(q, i); ok {
				end := strings.Index(q[i+len(tag):], tag)
				if end < 0 {
					i = len(q)
				} else {
					i += len(tag) + end + len(tag)
				}
				emit("?")
				continue
			}
			j := i + 1
			for j < len(q) && isDigit(q[j]) {
				j++
			}
			emit(q[i:j]) // marcador de parâmetro, como $1
			i = j
		case isDigit(c) && !identBefore(q, i):
			j := i
			for j < len(q) && (isDigit(q[j]) || q[j] == '.' || q[j] == 'e' || q[j] == 'E' ||
				((q[j] == '+' || q[j] == '-') && (q[j-1] == 'e' || q[j-1] == 'E'))) {
				j++
			}
			emit("?")
			i = j
		default:
			j := i + 1
			if isIdent(c) {
				for j < len(q) && isIdent(q[j]) {
					j++
				}
			}
			emit(q[i:j])
			i = j
		}
	}
	out := rowsRe.ReplaceAllString(listRe.ReplaceAllString(b.String(), "(?)"), "(?)")
	if len(out) > maxQueryLen {
		out = out[:maxQueryLen]
	}
	return out
}

// skipString devolve a posição depois do literal que começa em q[i] == '\”,
// tratando ” e \' como aspas escapadas.
func skipString(q string, i int) int {
	for i++; i < len(q); i++ {
		switch q[i] {
		case '\\':
			i++
		case '\'':
			if i+1 < len(q) && q[i+1] == '\'' {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(q)
}

// dollarTag reconhece o início de um literal entre cifrões, como $$ ou $tag$.
func dollarTag(q string, i int) (string, bool) {
	j := i + 1
	for j < len(q) && (isIdent(q[j]) && !isDigit(q[j]) || j > i+1 && isDigit(q[j])) {
		j++
	}
	if j < len(q) && q[j] == '$' {
		return q[i : j+1], true
	}
	return "", false
}

func identBefore(q string, i int) bool { return i > 0 && (isIdent(q[i-1]) || q[i-1] == '$') }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isIdent(c byte) bool {
	return c == '_' || isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'z') || c >= 0x80
}

var kinds = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "BEGIN", "COMMIT", "ROLLBACK",
	"CREATE", "ALTER", "DROP", "TRUNCATE", "COPY", "SET", "SHOW", "EXPLAIN", "VACUUM", "ANALYZE"}

// QueryKind devolve o comando da consulta, como "SELECT", ou "OUTRO".
func QueryKind(q string) string {
	q = strings.TrimLeft(q, " (")
	end := strings.IndexFunc(q, func(r rune) bool { return r == ' ' || r == '(' || r == ';' })
	if end < 0 {
		end = len(q)
	}
	word := strings.ToUpper(q[:end])
	for _, k := range kinds {
		if word == k {
			return k
		}
	}
	return "OUTRO"
}
