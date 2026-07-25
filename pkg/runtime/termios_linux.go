//go:build linux

package runtime

import "syscall"

// The termios struct on linux carries 32-bit flag words and 32-bit speeds; the
// packing in termiosmod.go converts through these aliases so the same code works
// against the wider darwin struct.
type tcflag = uint32
type ccval = uint8
type speedval = uint32

// termiosReadReq is the read-attributes ioctl. On linux it is TCGETS.
const termiosReadReq = syscall.TCGETS

// Go's linux syscall package defines TCSETS but not the drain and flush variants;
// they are the next two request numbers, fixed by the kernel ABI.
const (
	tcsetsw = 0x5403
	tcsetsf = 0x5404
)

// termiosWriteReq maps a tcsetattr "when" to the linux write-attributes ioctl:
// TCSETS takes effect now, TCSETSW after the output drains, TCSETSF after the
// output drains and the input is flushed.
func termiosWriteReq(when int) (uintptr, bool) {
	switch when {
	case tcsaNow:
		return syscall.TCSETS, true
	case tcsaDrain:
		return tcsetsw, true
	case tcsaFlush:
		return tcsetsf, true
	}
	return 0, false
}
