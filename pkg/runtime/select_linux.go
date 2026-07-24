//go:build linux

package runtime

import "syscall"

// On Linux an FdSet is a bitmap of int64 words and select() returns a ready
// count alongside the error, so these helpers work the fd bit in a 64-bit word
// and drop the count, which the caller recovers by re-scanning the sets.

func fdZero(s *syscall.FdSet) {
	for i := range s.Bits {
		s.Bits[i] = 0
	}
}

func fdSet(s *syscall.FdSet, fd int) {
	s.Bits[fd/64] |= int64(uint64(1) << (uint(fd) % 64))
}

func fdIsSet(s *syscall.FdSet, fd int) bool {
	return s.Bits[fd/64]&int64(uint64(1)<<(uint(fd)%64)) != 0
}

func sysSelect(nfd int, r, w, e *syscall.FdSet, tv *syscall.Timeval) error {
	_, err := syscall.Select(nfd, r, w, e, tv)
	return err
}
