//go:build !cgo

package sniffer

import "errors"

// OpenLive precisa do libpcap, disponível só em binários compilados com cgo.
func OpenLive(string, string) (Source, error) {
	return nil, errors.New("captura ao vivo indisponível: binário compilado sem cgo")
}
