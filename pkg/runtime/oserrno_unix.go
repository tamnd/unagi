//go:build !windows

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// This file wires the errno -> OSError-subclass mapping CPython applies when a
// bare OSError is constructed with an integer errno (Objects/exceptions.c's
// _PyErr_SetObjectAndCleanup / oserror_use_init path). Keeping it here, tagged
// to POSIX, lets pkg/objects stay platform-independent: it names the subclasses,
// this file supplies the errno numbers from package syscall. Windows selects the
// same subclasses off winerror rather than errno; that mapping is a separate
// concern and is left unwired there, where a bare OSError simply keeps its class.
func init() { objects.OSErrorSubclass = osErrnoSubclass }

func osErrnoSubclass(code int64) (string, bool) {
	// EWOULDBLOCK equals EAGAIN on every supported host, so it is covered by the
	// EAGAIN case and must not be listed again — a duplicate switch constant is a
	// compile error. The set mirrors CPython's _PyExc_InitState table.
	switch syscall.Errno(code) {
	case syscall.EAGAIN, syscall.EALREADY, syscall.EINPROGRESS:
		return "BlockingIOError", true
	case syscall.EPIPE, syscall.ESHUTDOWN:
		return "BrokenPipeError", true
	case syscall.ECHILD:
		return "ChildProcessError", true
	case syscall.ECONNABORTED:
		return "ConnectionAbortedError", true
	case syscall.ECONNREFUSED:
		return "ConnectionRefusedError", true
	case syscall.ECONNRESET:
		return "ConnectionResetError", true
	case syscall.EEXIST:
		return "FileExistsError", true
	case syscall.ENOENT:
		return "FileNotFoundError", true
	case syscall.EINTR:
		return "InterruptedError", true
	case syscall.EISDIR:
		return "IsADirectoryError", true
	case syscall.ENOTDIR:
		return "NotADirectoryError", true
	case syscall.EACCES, syscall.EPERM:
		return "PermissionError", true
	case syscall.ESRCH:
		return "ProcessLookupError", true
	case syscall.ETIMEDOUT:
		return "TimeoutError", true
	}
	return "", false
}
