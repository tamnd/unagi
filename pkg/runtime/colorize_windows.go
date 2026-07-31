//go:build windows

package runtime

import "syscall"

// colorizeFdIsCharDevice reports whether fd refers to a console, the terminal
// test can_colorize applies to a stream's descriptor. A console answers
// GetConsoleMode; a file, pipe or NUL does not. It queries the handle in place
// rather than wrapping it in an *os.File, whose finalizer would close this
// borrowed descriptor once collected.
func colorizeFdIsCharDevice(fd int) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(fd), &mode) == nil
}
