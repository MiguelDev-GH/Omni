//go:build unix

package security

import (
	"io/fs"
	"strconv"
	"syscall"
)

// owner devolve "uid:gid" do arquivo.
func owner(info fs.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return strconv.FormatUint(uint64(st.Uid), 10) + ":" + strconv.FormatUint(uint64(st.Gid), 10)
}
