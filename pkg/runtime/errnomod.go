package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// errno is the builtin that names the system error numbers os, io and socket
// code raises and inspects. It exposes each name as an integer attribute plus
// errorcode, the number->name dict. The numbers are platform-specific (EAGAIN is
// 35 on darwin, 11 on Linux, and Windows keeps its own table where the socket
// names live in the 10000 WSA range), so the name->number table itself is
// per-GOOS: errnonames_unix.go derives it from Go's syscall constants the way
// CPython's errno takes them from the build host's C headers, and
// errnonames_windows.go pins the MS C runtime / winsock values CPython reports on
// Windows. Both files declare errnoNames; this file only turns it into module
// attributes and the errorcode dict.
//
// errorcode maps a number back to its canonical name. Where two names share a
// number (EWOULDBLOCK aliases EAGAIN on POSIX; every WSAE* name aliases its E*
// twin on Windows), the first listed wins, matching CPython, which registers the
// canonical name before the alias. Each per-OS table is ordered so the name
// CPython keeps in errorcode comes first.

func init() {
	moduleTable["errno"] = &moduleEntry{builtin: true, exec: initErrno}
}

// errnoEntry is one name->number row of the per-OS errno table.
type errnoEntry struct {
	name string
	num  int
}

func initErrno(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	// errorcode is number->name; the first name for a shared number wins so an
	// alias never displaces its canonical name.
	errorcode, err := objects.NewDict(nil, nil)
	if err != nil {
		return err
	}
	seen := make(map[int]bool, len(errnoNames))
	for _, e := range errnoNames {
		if err := set(e.name, objects.NewInt(int64(e.num))); err != nil {
			return err
		}
		if !seen[e.num] {
			seen[e.num] = true
			if err := objects.SetItem(errorcode, objects.NewInt(int64(e.num)), objects.NewStr(e.name)); err != nil {
				return err
			}
		}
	}
	return set("errorcode", errorcode)
}
