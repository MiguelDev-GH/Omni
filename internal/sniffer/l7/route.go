package l7

import "strings"

const maxRouteLen = 200

// NormalizeRoute agrupa caminhos que só diferem por identificadores: tira a
// query string e troca por ":id" os segmentos numéricos, UUIDs, hexadecimais
// com 16 ou mais caracteres e tokens longos que misturam letras e dígitos.
func NormalizeRoute(path string) string {
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	if path == "" || path[0] != '/' {
		return "/"
	}
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if isID(s) {
			segs[i] = ":id"
		}
	}
	route := strings.Join(segs, "/")
	if len(route) > maxRouteLen {
		route = route[:maxRouteLen]
	}
	return route
}

func isID(s string) bool {
	if s == "" {
		return false
	}
	var digits, letters, hex int
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			digits++
			hex++
		case c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
			letters++
			hex++
		case c >= 'g' && c <= 'z', c >= 'G' && c <= 'Z':
			letters++
		case c == '-' || c == '_':
		default:
			return false
		}
	}
	switch {
	case digits == len(s):
		return true
	case isUUID(s):
		return true
	case hex == len(s) && len(s) >= 16:
		return true
	default:
		return len(s) >= 24 && digits > 0 && letters > 0
	}
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
