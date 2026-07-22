//go:build linux

package runtime

import "syscall"

// ioctlReadTermios is the terminal-attributes ioctl request isatty probes with.
// On Linux it is TCGETS.
const ioctlReadTermios = syscall.TCGETS

// fdDup2 points fd2 at fd's open file. Linux dropped the dup2 syscall on newer
// ports (arm64) in favor of dup3, so route through dup3. dup3 rejects fd == fd2
// with EINVAL where dup2 treats it as a validated no-op returning fd2, so keep
// that case: a bad fd still has to raise, so probe it with fstat first.
func fdDup2(fd, fd2 int) error {
	if fd == fd2 {
		var st syscall.Stat_t
		return syscall.Fstat(fd, &st)
	}
	return syscall.Dup3(fd, fd2, 0)
}
