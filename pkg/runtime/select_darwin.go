//go:build darwin

package runtime

import "syscall"

// On darwin an FdSet is a bitmap of int32 words and select() returns only an
// error, so these helpers work the fd bit in a 32-bit word and drop the ready
// count, which the caller recovers by re-scanning the sets.

func fdZero(s *syscall.FdSet) {
	for i := range s.Bits {
		s.Bits[i] = 0
	}
}

func fdSet(s *syscall.FdSet, fd int) {
	s.Bits[fd/32] |= int32(uint32(1) << (uint(fd) % 32))
}

func fdIsSet(s *syscall.FdSet, fd int) bool {
	return s.Bits[fd/32]&int32(uint32(1)<<(uint(fd)%32)) != 0
}

func sysSelect(nfd int, r, w, e *syscall.FdSet, tv *syscall.Timeval) error {
	return syscall.Select(nfd, r, w, e, tv)
}
