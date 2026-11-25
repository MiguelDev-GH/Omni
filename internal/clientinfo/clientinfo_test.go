package clientinfo

import (
	"encoding/base64"
	"testing"
)

func TestOriginIP(t *testing.T) {
	cases := []struct{ peer, xff, want string }{
		{"10.0.0.5", "", "10.0.0.5"},
		{"127.0.0.1", "", "127.0.0.1"},
		{"127.0.0.1", "203.0.113.9", "203.0.113.9"},
		{"::1", "2001:db8::7", "2001:db8::7"},
		// O cliente escreveu "1.1.1.1"; o proxy acrescentou o IP real no fim.
		{"127.0.0.1", "1.1.1.1, 203.0.113.9", "203.0.113.9"},
		// Conexão direta de fora: o cabeçalho é do cliente e é ignorado.
		{"198.51.100.4", "127.0.0.1", "198.51.100.4"},
		{"127.0.0.1", "lixo", "127.0.0.1"},
		{"127.0.0.1", "203.0.113.9, ", "127.0.0.1"},
	}
	for _, c := range cases {
		if got := OriginIP(c.peer, c.xff); got != c.want {
			t.Errorf("OriginIP(%q, %q) = %q, quer %q", c.peer, c.xff, got, c.want)
		}
	}
}

func basic(s string) string { return "Basic " + base64.StdEncoding.EncodeToString([]byte(s)) }

func TestBasicUser(t *testing.T) {
	cases := map[string]string{
		basic("ana:segredo"):                                          "ana",
		basic("ana:senha:com:dois-pontos"):                            "ana",
		"basic " + base64.StdEncoding.EncodeToString([]byte("bia:x")): "bia",
		basic(":semusuario"):                                          "",
		basic("semdoispontos"):                                        "",
		basic("a\x00b:x"):                                             "",
		basic(string(make([]byte, 65)) + ":x"):                        "",
		"Bearer abc.def":                                              "",
		"Basic %%%":                                                   "",
		"":                                                            "",
	}
	for in, want := range cases {
		if got := BasicUser(in); got != want {
			t.Errorf("BasicUser(%q) = %q, quer %q", in, got, want)
		}
	}
}
