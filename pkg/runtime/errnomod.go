package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// errno is the builtin that names the system error numbers os, io and socket
// code raises and inspects. It exposes each name as an integer attribute plus
// errorcode, the number->name dict. The numbers are platform-specific (EAGAIN is
// 35 on darwin, 11 on Linux), so they come from Go's syscall constants, resolved
// per-GOOS at compile time, the same way CPython's errno module takes them from
// the C headers of the build host. The name list is fixed here since Go has no
// reflection over constant names; it is the common POSIX surface, and any name
// absent on a host would fail the build, so it stays to the portable set.
//
// errorcode maps a number back to its canonical name. Where two names share a
// number (EWOULDBLOCK aliases EAGAIN), the first listed wins, matching CPython,
// which registers the canonical name before the alias.

func init() {
	moduleTable["errno"] = &moduleEntry{builtin: true, exec: initErrno}
}

// errnoNames is the name->number table, canonical names before their aliases so
// the first mapping to a shared number is the one errorcode keeps.
var errnoNames = []struct {
	name string
	num  int
}{
	{"EPERM", int(syscall.EPERM)},
	{"ENOENT", int(syscall.ENOENT)},
	{"ESRCH", int(syscall.ESRCH)},
	{"EINTR", int(syscall.EINTR)},
	{"EIO", int(syscall.EIO)},
	{"ENXIO", int(syscall.ENXIO)},
	{"E2BIG", int(syscall.E2BIG)},
	{"ENOEXEC", int(syscall.ENOEXEC)},
	{"EBADF", int(syscall.EBADF)},
	{"ECHILD", int(syscall.ECHILD)},
	{"EAGAIN", int(syscall.EAGAIN)},
	{"ENOMEM", int(syscall.ENOMEM)},
	{"EACCES", int(syscall.EACCES)},
	{"EFAULT", int(syscall.EFAULT)},
	{"ENOTBLK", int(syscall.ENOTBLK)},
	{"EBUSY", int(syscall.EBUSY)},
	{"EEXIST", int(syscall.EEXIST)},
	{"EXDEV", int(syscall.EXDEV)},
	{"ENODEV", int(syscall.ENODEV)},
	{"ENOTDIR", int(syscall.ENOTDIR)},
	{"EISDIR", int(syscall.EISDIR)},
	{"EINVAL", int(syscall.EINVAL)},
	{"ENFILE", int(syscall.ENFILE)},
	{"EMFILE", int(syscall.EMFILE)},
	{"ENOTTY", int(syscall.ENOTTY)},
	{"ETXTBSY", int(syscall.ETXTBSY)},
	{"EFBIG", int(syscall.EFBIG)},
	{"ENOSPC", int(syscall.ENOSPC)},
	{"ESPIPE", int(syscall.ESPIPE)},
	{"EROFS", int(syscall.EROFS)},
	{"EMLINK", int(syscall.EMLINK)},
	{"EPIPE", int(syscall.EPIPE)},
	{"EDOM", int(syscall.EDOM)},
	{"ERANGE", int(syscall.ERANGE)},
	{"EDEADLK", int(syscall.EDEADLK)},
	{"ENAMETOOLONG", int(syscall.ENAMETOOLONG)},
	{"ENOLCK", int(syscall.ENOLCK)},
	{"ENOSYS", int(syscall.ENOSYS)},
	{"ENOTEMPTY", int(syscall.ENOTEMPTY)},
	{"ELOOP", int(syscall.ELOOP)},
	{"EWOULDBLOCK", int(syscall.EWOULDBLOCK)},
	{"ENOMSG", int(syscall.ENOMSG)},
	{"EIDRM", int(syscall.EIDRM)},
	{"EOVERFLOW", int(syscall.EOVERFLOW)},
	{"EILSEQ", int(syscall.EILSEQ)},
	{"ENOTSOCK", int(syscall.ENOTSOCK)},
	{"EDESTADDRREQ", int(syscall.EDESTADDRREQ)},
	{"EMSGSIZE", int(syscall.EMSGSIZE)},
	{"EPROTOTYPE", int(syscall.EPROTOTYPE)},
	{"ENOPROTOOPT", int(syscall.ENOPROTOOPT)},
	{"EPROTONOSUPPORT", int(syscall.EPROTONOSUPPORT)},
	{"EOPNOTSUPP", int(syscall.EOPNOTSUPP)},
	{"EAFNOSUPPORT", int(syscall.EAFNOSUPPORT)},
	{"EADDRINUSE", int(syscall.EADDRINUSE)},
	{"EADDRNOTAVAIL", int(syscall.EADDRNOTAVAIL)},
	{"ENETDOWN", int(syscall.ENETDOWN)},
	{"ENETUNREACH", int(syscall.ENETUNREACH)},
	{"ENETRESET", int(syscall.ENETRESET)},
	{"ECONNABORTED", int(syscall.ECONNABORTED)},
	{"ECONNRESET", int(syscall.ECONNRESET)},
	{"ENOBUFS", int(syscall.ENOBUFS)},
	{"EISCONN", int(syscall.EISCONN)},
	{"ENOTCONN", int(syscall.ENOTCONN)},
	{"ETIMEDOUT", int(syscall.ETIMEDOUT)},
	{"ECONNREFUSED", int(syscall.ECONNREFUSED)},
	{"EHOSTDOWN", int(syscall.EHOSTDOWN)},
	{"EHOSTUNREACH", int(syscall.EHOSTUNREACH)},
	{"EALREADY", int(syscall.EALREADY)},
	{"EINPROGRESS", int(syscall.EINPROGRESS)},
	{"ESTALE", int(syscall.ESTALE)},
	{"EDQUOT", int(syscall.EDQUOT)},
	{"ECANCELED", int(syscall.ECANCELED)},
	{"EOWNERDEAD", int(syscall.EOWNERDEAD)},
	{"ENOTRECOVERABLE", int(syscall.ENOTRECOVERABLE)},
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
