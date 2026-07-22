//go:build darwin

package runtime

import "syscall"

// ioctlReadTermios is the terminal-attributes ioctl request isatty probes with.
// On darwin it is TIOCGETA.
const ioctlReadTermios = syscall.TIOCGETA

// fdDup2 points fd2 at fd's open file. darwin keeps the classic dup2.
func fdDup2(fd, fd2 int) error { return syscall.Dup2(fd, fd2) }
