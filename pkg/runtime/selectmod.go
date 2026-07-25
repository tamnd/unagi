//go:build !windows

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// select is the I/O multiplexing accelerator selectors.py runs on. selectors
// imports select and, at import time, probes select for poll/epoll/devpoll/
// kqueue with hasattr and falls back to select.select when none are present.
// This module exposes only select.select, the one primitive POSIX guarantees
// everywhere, so selectors resolves its DefaultSelector to SelectSelector on
// every host and the pure-Python selector logic runs on top.
//
// select.select waits until one of the given file descriptors is ready, then
// returns the subset of each input list that is ready. The fd_set manipulation
// and the syscall signature differ across hosts (darwin's FdSet words are int32
// and its Select returns only an error, Linux's are int64 and it returns a
// count too), so fdZero/fdSet/fdIsSet and sysSelect are provided per-GOOS and
// this file holds only the portable argument handling.

// fdSetSize is FD_SETSIZE, the largest fd count an fd_set holds. select cannot
// name an fd at or above it, matching CPython's own bound.
const fdSetSize = 1024

func init() {
	moduleTable["select"] = &moduleEntry{builtin: true, exec: initSelect}
}

func initSelect(m *objects.Module) error {
	if err := objects.StoreAttr(m, "select", objects.NewFunc("select", -1, selectSelect)); err != nil {
		return err
	}
	// select.error is the module's historical spelling of OSError, the class its
	// calls raise, still caught by older code as select.error.
	if oserr, ok := objects.ExcClassValue("OSError"); ok {
		if err := objects.StoreAttr(m, "error", oserr); err != nil {
			return err
		}
	}
	return nil
}

// selectFd pairs an input object with the fd it names, so the ready lists can
// return the original objects the caller passed rather than bare fds.
type selectFd struct {
	obj objects.Object
	fd  int
}

// selectFdOf reads the fd an object names: an integer stands for itself, and any
// other object is asked for its fileno(), the way CPython's select accepts a
// socket or file.
func selectFdOf(o objects.Object) (int, error) {
	if v, ok := objects.AsInt(o); ok {
		return int(v), nil
	}
	r, err := objects.CallMethod(o, "fileno", nil)
	if err != nil {
		return 0, objects.Raise(objects.TypeError, "argument must be an int, or have a fileno() method.")
	}
	v, ok := objects.AsInt(r)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "fileno() returned a non-integer")
	}
	return int(v), nil
}

// selectParseList reads one of the three argument sequences into fd pairs,
// validating each fd against the fd_set bound the way CPython does.
func selectParseList(seq objects.Object) ([]selectFd, error) {
	it, err := objects.Iter(seq)
	if err != nil {
		return nil, err
	}
	var out []selectFd
	for {
		item, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		fd, err := selectFdOf(item)
		if err != nil {
			return nil, err
		}
		if fd < 0 {
			return nil, objects.Raise(objects.ValueError, "file descriptor cannot be a negative integer (%d)", fd)
		}
		if fd >= fdSetSize {
			return nil, objects.Raise(objects.ValueError, "filedescriptor out of range in select()")
		}
		out = append(out, selectFd{obj: item, fd: fd})
	}
	return out, nil
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

// selectSelect is select.select(rlist, wlist, xlist, timeout=None): it blocks
// until a listed fd is ready to read, ready to write, or has an exceptional
// condition, or until the timeout elapses, then returns the three ready
// sublists. A None or absent timeout blocks indefinitely; a number is a
// non-negative count of seconds.
func selectSelect(args []objects.Object) (objects.Object, error) {
	if len(args) < 3 || len(args) > 4 {
		return nil, objects.Raise(objects.TypeError, "select expected 3 to 4 arguments, got %d", len(args))
	}
	rEntries, err := selectParseList(args[0])
	if err != nil {
		return nil, err
	}
	wEntries, err := selectParseList(args[1])
	if err != nil {
		return nil, err
	}
	eEntries, err := selectParseList(args[2])
	if err != nil {
		return nil, err
	}

	var tv *syscall.Timeval
	if len(args) == 4 && args[3] != objects.None {
		secs, ok := objects.AsFloat(args[3])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "timeout must be a float or None")
		}
		if secs < 0 {
			return nil, objects.Raise(objects.ValueError, "timeout must be non-negative")
		}
		t := syscall.NsecToTimeval(int64(secs * 1e9))
		tv = &t
	}

	var rset, wset, eset syscall.FdSet
	fdZero(&rset)
	fdZero(&wset)
	fdZero(&eset)
	nfd := 0
	for _, e := range rEntries {
		fdSet(&rset, e.fd)
		if e.fd+1 > nfd {
			nfd = e.fd + 1
		}
	}
	for _, e := range wEntries {
		fdSet(&wset, e.fd)
		if e.fd+1 > nfd {
			nfd = e.fd + 1
		}
	}
	for _, e := range eEntries {
		fdSet(&eset, e.fd)
		if e.fd+1 > nfd {
			nfd = e.fd + 1
		}
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
