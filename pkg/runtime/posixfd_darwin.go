//go:build darwin

package runtime

import "syscall"

// ioctlReadTermios is the terminal-attributes ioctl request isatty probes with.
// On darwin it is TIOCGETA.
const ioctlReadTermios = syscall.TIOCGETA

// fdDup2 points fd2 at fd's open file. darwin keeps the classic dup2.
func fdDup2(fd, fd2 int) error { return syscall.Dup2(fd, fd2) }

// pipeCloexec creates a pipe whose two fds are close-on-exec, matching the
// non-inheritable fds CPython's os.pipe returns. darwin has no atomic pipe2, so
// it does the two-step under ForkLock, the same guard Go's own os.Pipe uses so a
// concurrent fork cannot leak the fds before the flag is set.
func pipeCloexec(fds *[2]int) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	if err := syscall.Pipe(fds[:]); err != nil {
		return err
	}
	syscall.CloseOnExec(fds[0])
	syscall.CloseOnExec(fds[1])
	return nil
}
