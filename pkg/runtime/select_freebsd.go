//go:build freebsd

package runtime

import "syscall"

// On freebsd an FdSet is a bitmap of uint64 words (X__fds_bits) and select()
// returns only an error, so these helpers work the fd bit in a 64-bit word and
// drop the ready count, which the caller recovers by re-scanning the sets.

func fdZero(s *syscall.FdSet) {
	for i := range s.X__fds_bits {
		s.X__fds_bits[i] = 0
	}
}

func fdSet(s *syscall.FdSet, fd int) {
	s.X__fds_bits[fd/64] |= uint64(1) << (uint(fd) % 64)
}

func fdIsSet(s *syscall.FdSet, fd int) bool {
	return s.X__fds_bits[fd/64]&(uint64(1)<<(uint(fd)%64)) != 0
}

func sysSelect(nfd int, r, w, e *syscall.FdSet, tv *syscall.Timeval) error {
	return syscall.Select(nfd, r, w, e, tv)
}
