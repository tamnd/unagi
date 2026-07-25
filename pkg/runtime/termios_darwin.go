//go:build darwin

package runtime

import "syscall"

// The termios struct on darwin carries 64-bit flag words and 64-bit speeds; the
// packing in termiosmod.go converts through these aliases so the same code works
// against the narrower linux struct.
type tcflag = uint64
type ccval = uint8
type speedval = uint64

// termiosReadReq is the read-attributes ioctl. On darwin it is TIOCGETA.
const termiosReadReq = syscall.TIOCGETA

// termiosWriteReq maps a tcsetattr "when" to the darwin write-attributes ioctl:
// TIOCSETA takes effect now, TIOCSETAW after the output drains, TIOCSETAF after
// the output drains and the input is flushed.
func termiosWriteReq(when int) (uintptr, bool) {
	switch when {
	case tcsaNow:
		return syscall.TIOCSETA, true
	case tcsaDrain:
		return syscall.TIOCSETAW, true
	case tcsaFlush:
		return syscall.TIOCSETAF, true
	}
	return 0, false
}
