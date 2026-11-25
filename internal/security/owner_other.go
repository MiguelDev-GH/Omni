//go:build !unix

package security

import "io/fs"

// owner não é suportado fora de sistemas Unix.
func owner(fs.FileInfo) string { return "" }
