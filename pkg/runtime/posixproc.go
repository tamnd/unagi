package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// posix process-wait surface: os.py re-exports waitpid, the W* status macros,
// waitstatus_to_exitcode and the fd-inheritance calls via `from posix import *`,
// and subprocess.Popen drives them to reap children and turn a raw wait status
// into a returncode. The status integer is the platform's raw wait status; the
// macros read it through syscall.WaitStatus, whose bit layout matches the host
// libc the CPython macros use, so the answers agree on both darwin and linux.
//
// The WNOHANG/WUNTRACED/WCONTINUED option flags come from syscall so each host
// gets its own value, the same way the open flags and errno numbers do.

func initPosixProc(set func(string, objects.Object) error) error {
	for name, val := range map[string]int{
		"WNOHANG":    syscall.WNOHANG,
		"WUNTRACED":  syscall.WUNTRACED,
		"WCONTINUED": syscall.WCONTINUED,
	} {
		if err := set(name, objects.NewInt(int64(val))); err != nil {
			return err
		}
	}
	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"waitpid", posixWaitpid},
		{"waitstatus_to_exitcode", posixWaitstatusToExitcode},
		{"WIFEXITED", posixWIFEXITED},
		{"WEXITSTATUS", posixWEXITSTATUS},
		{"WIFSIGNALED", posixWIFSIGNALED},
		{"WTERMSIG", posixWTERMSIG},
		{"WIFSTOPPED", posixWIFSTOPPED},
		{"WSTOPSIG", posixWSTOPSIG},
		{"WIFCONTINUED", posixWIFCONTINUED},
		{"WCOREDUMP", posixWCOREDUMP},
		{"set_inheritable", posixSetInheritable},
		{"get_inheritable", posixGetInheritable},
		{"closerange", posixCloserange},
		{"kill", posixKill},
		{"killpg", posixKillpg},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// posixWaitErr maps a wait error to the exception CPython raises: a child that
// has already been reaped (or was never ours) is ECHILD, which surfaces as
// ChildProcessError, the OSError subclass subprocess catches. Anything else
// falls through to the shared path.
func posixWaitErr(err error) error {
	if errno, ok := err.(syscall.Errno); ok && errno == syscall.ECHILD {
		return objects.Raise("ChildProcessError", "[Errno %d] %s", int(errno), errno.Error())
	}
	return posixStatErr(err)
}

// posixWaitpid is posix.waitpid(pid, options): it waits for a child and returns
// the (pid, status) pair, where status is the raw wait status the W* macros and
// waitstatus_to_exitcode decode. options is 0 for a blocking wait or WNOHANG to
// poll (returning (0, 0) when no child has changed state).
func posixWaitpid(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "waitpid() takes exactly 2 arguments (%d given)", len(args))
	}
	pid, err := posixArgInt("waitpid", args, 0)
	if err != nil {
		return nil, err
	}
	options, err := posixArgInt("waitpid", args, 1)
	if err != nil {
		return nil, err
	}
	var ws syscall.WaitStatus
	wpid, serr := syscall.Wait4(pid, &ws, options, nil)
	if serr != nil {
		return nil, posixWaitErr(serr)
	}
	return objects.NewTuple([]objects.Object{
		objects.NewInt(int64(wpid)),
		objects.NewInt(int64(ws)),
	}), nil
}

// waitStatusArg reads the single raw-status integer the W* macros take. It is a
// plain int, decoded by the bit math below rather than syscall.WaitStatus's
// methods: Go's darwin Stopped() special-cases SIGSTOP, which diverges from
// CPython's WIFSTOPPED. The <sys/wait.h> macro layout is identical on darwin and
// linux, so open-coding it keeps the answers byte-exact with CPython on both.
func waitStatusArg(name string, args []objects.Object) (int, error) {
	if len(args) != 1 {
		return 0, objects.Raise(objects.TypeError, "%s() takes exactly 1 argument (%d given)", name, len(args))
	}
	// AsIntValue unwraps int subclasses (e.g. a signal.Signals member folded into
	// a status), matching CPython, where the macros take any int.
	s, ok := objects.AsIntValue(args[0])
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	return int(s), nil
}

// The <sys/wait.h> status macros, open-coded to match CPython exactly. The low 7
// bits hold the term signal (0 means a normal exit, 0x7f a stop), bit 0x80 is
// the core-dump flag, and the exit or stop signal sits in bits 8+.
func wIfExited(s int) bool    { return s&0x7f == 0 }
func wExitStatus(s int) int   { return (s >> 8) & 0xff }
func wIfSignaled(s int) bool  { return int8((s&0x7f)+1)>>1 > 0 }
func wTermSig(s int) int      { return s & 0x7f }
func wIfStopped(s int) bool   { return s&0xff == 0x7f }
func wStopSig(s int) int      { return (s >> 8) & 0xff }
func wCoreDump(s int) bool    { return s&0x80 != 0 }
func wIfContinued(s int) bool { return s == 0xffff }

// posixWaitstatusToExitcode turns a raw wait status into the exit code shell
// conventions use: a normal exit gives its code, a killed process gives the
// negated signal number, and a stopped process is an error, matching CPython.
func posixWaitstatusToExitcode(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("waitstatus_to_exitcode", args)
	if err != nil {
		return nil, err
	}
	switch {
	case wIfExited(s):
		return objects.NewInt(int64(wExitStatus(s))), nil
	case wIfSignaled(s):
		return objects.NewInt(int64(-wTermSig(s))), nil
	case wIfStopped(s):
		return nil, objects.Raise(objects.ValueError, "process stopped by delivery of signal %d", wStopSig(s))
	default:
		return nil, objects.Raise(objects.ValueError, "invalid wait status: %d", s)
	}
}

func posixWIFEXITED(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WIFEXITED", args)
	if err != nil {
		return nil, err
	}
	return objects.NewBool(wIfExited(s)), nil
}

func posixWEXITSTATUS(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WEXITSTATUS", args)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(wExitStatus(s))), nil
}

func posixWIFSIGNALED(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WIFSIGNALED", args)
	if err != nil {
		return nil, err
	}
	return objects.NewBool(wIfSignaled(s)), nil
}

func posixWTERMSIG(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WTERMSIG", args)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(wTermSig(s))), nil
}

func posixWIFSTOPPED(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WIFSTOPPED", args)
	if err != nil {
		return nil, err
	}
	return objects.NewBool(wIfStopped(s)), nil
}

func posixWSTOPSIG(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WSTOPSIG", args)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(wStopSig(s))), nil
}

func posixWIFCONTINUED(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WIFCONTINUED", args)
	if err != nil {
		return nil, err
	}
	return objects.NewBool(wIfContinued(s)), nil
}

func posixWCOREDUMP(args []objects.Object) (objects.Object, error) {
	s, err := waitStatusArg("WCOREDUMP", args)
	if err != nil {
		return nil, err
	}
	return objects.NewBool(wCoreDump(s)), nil
}

// fcntlInt runs fcntl(fd, cmd, arg) through the raw syscall. The syscall package
// exposes FcntlFlock but not the int form, and x/sys is off limits to the
// dependency-free build module, so this goes straight to SYS_FCNTL, defined on
// both supported hosts.
func fcntlInt(fd uintptr, cmd, arg int) (int, error) {
	r, _, e := syscall.Syscall(syscall.SYS_FCNTL, fd, uintptr(cmd), uintptr(arg))
	if e != 0 {
		return int(r), e
	}
	return int(r), nil
}

// posixSetInheritable is posix.set_inheritable(fd, inheritable): it clears or
// sets the fd's close-on-exec flag so a child does or does not keep the fd
// across an exec. Inheritable means NOT close-on-exec, the inverse of the
// FD_CLOEXEC bit, matching CPython.
func posixSetInheritable(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "set_inheritable() takes exactly 2 arguments (%d given)", len(args))
	}
	fd, err := posixArgInt("set_inheritable", args, 0)
	if err != nil {
		return nil, err
	}
	inheritable, err := objects.TruthOf(args[1])
	if err != nil {
		return nil, err
	}
	flags, serr := fcntlInt(uintptr(fd), syscall.F_GETFD, 0)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	if inheritable {
		flags &^= syscall.FD_CLOEXEC
	} else {
		flags |= syscall.FD_CLOEXEC
	}
	if _, serr := fcntlInt(uintptr(fd), syscall.F_SETFD, flags); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.None, nil
}

// posixGetInheritable is posix.get_inheritable(fd): True when the fd survives an
// exec, i.e. its close-on-exec flag is clear.
func posixGetInheritable(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "get_inheritable() takes exactly 1 argument (%d given)", len(args))
	}
	fd, err := posixArgInt("get_inheritable", args, 0)
	if err != nil {
		return nil, err
	}
	flags, serr := fcntlInt(uintptr(fd), syscall.F_GETFD, 0)
	if serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewBool(flags&syscall.FD_CLOEXEC == 0), nil
}

// posixKill is posix.kill(pid, sig): it sends signal sig to process pid, the
// call Popen.send_signal/terminate/kill drive to signal a child. sig is a plain
// int or a signal.Signals member (an int subclass), matching CPython.
func posixKill(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "kill() takes exactly 2 arguments (%d given)", len(args))
	}
	pid, err := posixArgInt("kill", args, 0)
	if err != nil {
		return nil, err
	}
	sig, ok := objects.AsIntValue(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	if serr := syscall.Kill(pid, syscall.Signal(sig)); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.None, nil
}

// posixKillpg is posix.killpg(pgid, sig): it sends signal sig to the whole
// process group pgid, used to signal a child started with start_new_session.
func posixKillpg(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "killpg() takes exactly 2 arguments (%d given)", len(args))
	}
	pgid, err := posixArgInt("killpg", args, 0)
	if err != nil {
		return nil, err
	}
	sig, ok := objects.AsIntValue(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	if serr := syscall.Kill(-pgid, syscall.Signal(sig)); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.None, nil
}

// posixCloserange is posix.closerange(fd_low, fd_high): it closes every fd in
// [fd_low, fd_high), ignoring fds that are not open, the same best-effort sweep
// CPython does.
func posixCloserange(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "closerange() takes exactly 2 arguments (%d given)", len(args))
	}
	low, err := posixArgInt("closerange", args, 0)
	if err != nil {
		return nil, err
	}
	high, err := posixArgInt("closerange", args, 1)
	if err != nil {
		return nil, err
	}
	for fd := low; fd < high; fd++ {
		_ = syscall.Close(fd)
	}
	return objects.None, nil
}
