//go:build !windows

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// selectImpl is the unix half of select.select: a bitmap fd_set indexed by fd
// and posix select(). The per-word bit operations (fdZero/fdSet/fdIsSet) and the
// syscall (sysSelect) are provided per-GOOS in select_darwin.go and
// select_linux.go, since darwin's fd_set words are int32 and its Select returns
// only an error while Linux's words are int64 and its Select returns a count.

// fdSetSize is FD_SETSIZE, the largest fd count a bitmap fd_set holds; select
// cannot name an fd at or above it, matching CPython's own bound. Windows bounds
// the socket count instead, with its own constant in select_windows.go.
const fdSetSize = 1024

func selectImpl(rEntries, wEntries, eEntries []selectFd, timeout *float64) (objects.Object, error) {
	var tv *syscall.Timeval
	if timeout != nil {
		t := syscall.NsecToTimeval(int64(*timeout * 1e9))
		tv = &t
	}

	var rset, wset, eset syscall.FdSet
	fdZero(&rset)
	fdZero(&wset)
	fdZero(&eset)
	nfd := 0
	build := func(entries []selectFd, set *syscall.FdSet) error {
		for _, e := range entries {
			// On unix an fd is a small index into the bitmap, so it must be a
			// non-negative value below FD_SETSIZE, matching CPython's checks.
			if e.fd < 0 {
				return objects.Raise(objects.ValueError, "file descriptor cannot be a negative integer (%d)", e.fd)
			}
			if e.fd >= fdSetSize {
				return objects.Raise(objects.ValueError, "filedescriptor out of range in select()")
			}
			fdSet(set, e.fd)
			if e.fd+1 > nfd {
				nfd = e.fd + 1
			}
		}
		return nil
	}
	if err := build(rEntries, &rset); err != nil {
		return nil, err
	}
	if err := build(wEntries, &wset); err != nil {
		return nil, err
	}
	if err := build(eEntries, &eset); err != nil {
		return nil, err
	}

	if err := sysSelect(nfd, &rset, &wset, &eset, tv); err != nil {
		return nil, posixStatErr(err)
	}

	return objects.NewTuple([]objects.Object{
		objects.NewList(selectReady(rEntries, &rset)),
		objects.NewList(selectReady(wEntries, &wset)),
		objects.NewList(selectReady(eEntries, &eset)),
	}), nil
}

// selectReady collects the original objects whose fd stayed set in the result
// fd_set, preserving the order they were passed in.
func selectReady(entries []selectFd, set *syscall.FdSet) []objects.Object {
	out := []objects.Object{}
	for _, e := range entries {
		if fdIsSet(set, e.fd) {
			out = append(out, e.obj)
		}
	}
	return out
}
