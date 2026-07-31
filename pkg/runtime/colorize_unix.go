//go:build !windows

package runtime

import "syscall"

// colorizeFdIsCharDevice reports whether fd refers to a character device, the
// terminal test can_colorize applies to a stream's descriptor. It fstats the
// descriptor in place rather than wrapping it in an *os.File, whose finalizer
// would close this borrowed descriptor once collected.
func colorizeFdIsCharDevice(fd int) bool {
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return false
	}
	return st.Mode&syscall.S_IFMT == syscall.S_IFCHR
}
