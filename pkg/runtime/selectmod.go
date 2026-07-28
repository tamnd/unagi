package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// select is the I/O multiplexing accelerator selectors.py runs on. selectors
// imports select and, at import time, probes select for poll/epoll/devpoll/
// kqueue with hasattr and falls back to select.select when none are present.
// This module exposes only select.select, the one primitive every host has, so
// selectors resolves its DefaultSelector to SelectSelector everywhere and the
// pure-Python selector logic runs on top. CPython on Windows likewise ships no
// poll/epoll in select, so the same fallback holds there.
//
// select.select waits until one of the given file descriptors is ready, then
// returns the subset of each input list that is ready. The fd_set layout and
// the syscall differ sharply across hosts: unix uses a bitmap indexed by fd and
// posix select(); Windows uses a winsock fd_set that is a counted array of
// SOCKET handles and ws2_32's select(). So the set representation, the bound
// check, and the call live per-GOOS behind selectImpl, and this file holds only
// the portable argument handling that is identical everywhere.

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

// selectParseList reads one of the three argument sequences into fd pairs. It
// only resolves each object to its fd; the fd_set bound check differs by host
// (unix bounds the fd value, Windows the socket count) and so lives in the
// per-GOOS selectImpl.
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
		out = append(out, selectFd{obj: item, fd: fd})
	}
	return out, nil
}

// selectSelect is select.select(rlist, wlist, xlist, timeout=None): it blocks
// until a listed fd is ready to read, ready to write, or has an exceptional
// condition, or until the timeout elapses, then returns the three ready
// sublists. A None or absent timeout blocks indefinitely; a number is a
// non-negative count of seconds. The set building and the wait are handed to the
// per-GOOS selectImpl.
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

	var timeout *float64
	if len(args) == 4 && args[3] != objects.None {
		secs, ok := objects.AsFloat(args[3])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "timeout must be a float or None")
		}
		if secs < 0 {
			return nil, objects.Raise(objects.ValueError, "timeout must be non-negative")
		}
		timeout = &secs
	}

	return selectImpl(rEntries, wEntries, eEntries, timeout)
}
