//go:build windows

package runtime

import (
	"syscall"
	"unsafe"

	"github.com/tamnd/unagi/pkg/objects"
)

// selectImpl is the Windows half of select.select. Windows has no bitmap fd_set:
// winsock's fd_set is a counted array of SOCKET handles, and only sockets can be
// waited on. So the layout, the bound (a cap on the number of sockets rather
// than on the fd value), and the call to ws2_32's select() are all their own
// here. CPython behaves the same way, so this stays faithful to it: a list
// longer than FD_SETSIZE raises ValueError, and any winsock failure surfaces as
// OSError. Note that winsock needs WSAStartup before select does anything; the
// socket module performs it, and until then select() reports WSANOTINITIALISED,
// exactly as CPython's select does when socket has not been imported.

// winFdSetSize is FD_SETSIZE as CPython builds it on Windows: it raises when a
// single list holds more than this many sockets.
const winFdSetSize = 512

// winFdSet mirrors winsock's fd_set: a count followed by that many SOCKET
// handles. A SOCKET is a UINT_PTR, so uintptr holds one exactly.
type winFdSet struct {
	count uint32
	array [winFdSetSize]uintptr
}

// winTimeval mirrors winsock's timeval, whose long fields are 32-bit on Windows.
type winTimeval struct {
	sec  int32
	usec int32
}

var (
	ws2_32              = syscall.NewLazyDLL("ws2_32.dll")
	procSelect          = ws2_32.NewProc("select")
	procWSAGetLastError = ws2_32.NewProc("WSAGetLastError")
)

// winBuildSet loads the fds into a winsock fd_set, enforcing the FD_SETSIZE cap
// on the count the way CPython's seq2set does. The set is always returned (never
// nil) so select receives a real, zeroed fd_set even for an empty list, matching
// CPython, which passes the address of a zero-count set rather than NULL.
func winBuildSet(entries []selectFd) (*winFdSet, error) {
	var s winFdSet
	for _, e := range entries {
		if s.count >= winFdSetSize {
			return nil, objects.Raise(objects.ValueError, "too many file descriptors in select()")
		}
		s.array[s.count] = uintptr(e.fd)
		s.count++
	}
	return &s, nil
}

// winFdIsSet reports whether an fd survived in the result set. Winsock rewrites
// each fd_set in place to hold only the ready sockets, so membership is a linear
// scan of the returned array, which is FD_ISSET on Windows.
func winFdIsSet(fd uintptr, s *winFdSet) bool {
	for i := uint32(0); i < s.count; i++ {
		if s.array[i] == fd {
			return true
		}
	}
	return false
}

func winSelectReady(entries []selectFd, s *winFdSet) []objects.Object {
	out := []objects.Object{}
	for _, e := range entries {
		if winFdIsSet(uintptr(e.fd), s) {
			out = append(out, e.obj)
		}
	}
	return out
}

func selectImpl(rEntries, wEntries, eEntries []selectFd, timeout *float64) (objects.Object, error) {
	rset, err := winBuildSet(rEntries)
	if err != nil {
		return nil, err
	}
	wset, err := winBuildSet(wEntries)
	if err != nil {
		return nil, err
	}
	eset, err := winBuildSet(eEntries)
	if err != nil {
		return nil, err
	}

	var tvp uintptr
	var tv winTimeval
	if timeout != nil {
		secs := *timeout
		tv.sec = int32(secs)
		tv.usec = int32((secs - float64(tv.sec)) * 1e6)
		tvp = uintptr(unsafe.Pointer(&tv))
	}

	// select(nfds, readfds, writefds, exceptfds, timeout). nfds is ignored on
	// Windows, so it is passed as zero.
	r0, _, _ := procSelect.Call(
		0,
		uintptr(unsafe.Pointer(rset)),
		uintptr(unsafe.Pointer(wset)),
		uintptr(unsafe.Pointer(eset)),
		tvp,
	)
	if int32(r0) == -1 { // SOCKET_ERROR
		code, _, _ := procWSAGetLastError.Call()
		return nil, objects.Raise("OSError", "[Errno %d] %s", int(int32(code)), "winsock select failed")
	}

	return objects.NewTuple([]objects.Object{
		objects.NewList(winSelectReady(rEntries, rset)),
		objects.NewList(winSelectReady(wEntries, wset)),
		objects.NewList(winSelectReady(eEntries, eset)),
	}), nil
}
