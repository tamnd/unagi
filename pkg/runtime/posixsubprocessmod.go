//go:build !windows

package runtime

import (
	"os"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// _posixsubprocess is the C accelerator subprocess.py runs on: `from
// _posixsubprocess import fork_exec` is the one call it needs, the primitive that
// launches a child with its stdio wired to pipes. Without it `import subprocess`
// fails at the import, which also walls off webbrowser and multiprocessing.
//
// CPython's fork_exec forks and then, in the child, does all the fd plumbing and
// exec by hand using only async-signal-safe calls. A compiled unagi program has
// no interpreter to run in a forked child and the Go runtime is not fork-safe, so
// this routes through syscall.ForkExec, which forks and execs atomically the way
// os/exec does: the child only runs the runtime's own async-signal-safe path. The
// fd wiring (stdin/out/err from the pipe ends) and the child attributes (new
// session, process group, credentials) map onto syscall.ProcAttr. On an exec
// failure ForkExec surfaces the errno in the parent, so this raises the mapped
// OSError directly instead of writing it back through subprocess's error pipe;
// subprocess.py propagates that exception unchanged.
//
// DIVERGENCES: preexec_fn cannot run (there is no child interpreter), so a
// non-None preexec_fn raises. restore_signals is always effectively on, because
// ForkExec resets the child's signal dispositions to default regardless. A umask
// other than the default -1 is not applied, since syscall.ProcAttr has no umask
// hook; the common Popen path leaves it -1.

func init() {
	moduleTable["_posixsubprocess"] = &moduleEntry{builtin: true, exec: initPosixSubprocess}
}

func initPosixSubprocess(m *objects.Module) error {
	return objects.StoreAttr(m, "fork_exec", objects.NewFunc("fork_exec", -1, posixForkExec))
}

// posixForkExec is _posixsubprocess.fork_exec, the 22-argument primitive
// subprocess.Popen._execute_child calls to spawn a child. It returns the child
// pid on success.
func posixForkExec(args []objects.Object) (objects.Object, error) {
	if len(args) != 22 {
		return nil, objects.Raise(objects.TypeError, "fork_exec() takes exactly 22 arguments (%d given)", len(args))
	}

	argv, err := forkExecStrSeq("args", args[0])
	if err != nil {
		return nil, err
	}
	execList, err := forkExecStrSeq("executable_list", args[1])
	if err != nil {
		return nil, err
	}
	if len(execList) == 0 {
		return nil, objects.Raise(objects.ValueError, "cannot find executable")
	}

	cwd := ""
	if args[4] != objects.None {
		s, ok := posixFsPath(args[4])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "cwd must be a string, bytes or None")
		}
		cwd = s
	}

	// env_list is None to inherit the parent environment (execv), else a list of
	// b"KEY=VALUE" entries (execve). ForkExec treats a nil Env as an empty
	// environment, so inherit means snapshotting os.Environ here.
	var env []string
	if args[5] == objects.None {
		env = os.Environ()
	} else {
		env, err = forkExecStrSeq("env_list", args[5])
		if err != nil {
			return nil, err
		}
	}

	fd := func(i int) (int, error) { return posixArgInt("fork_exec", args, i) }
	p2cread, err := fd(6)
	if err != nil {
		return nil, err
	}
	c2pwrite, err := fd(9)
	if err != nil {
		return nil, err
	}
	errwrite, err := fd(11)
	if err != nil {
		return nil, err
	}

	restoreSignals, err := objects.TruthOf(args[14])
	if err != nil {
		return nil, err
	}
	_ = restoreSignals // ForkExec always resets child signal dispositions.
	startNewSession, err := objects.TruthOf(args[15])
	if err != nil {
		return nil, err
	}
	processGroup, err := fd(16)
	if err != nil {
		return nil, err
	}

	if args[21] != objects.None {
		return nil, objects.Raise("RuntimeError", "preexec_fn is not supported in unagi compiled programs")
	}

	sys := &syscall.SysProcAttr{
		Setsid: startNewSession,
	}
	if processGroup >= 0 {
		sys.Setpgid = true
		sys.Pgid = processGroup
	}
	cred, err := forkExecCredential(args[17], args[18], args[19])
	if err != nil {
		return nil, err
	}
	sys.Credential = cred

	// stdin/out/err come from the child ends of the pipes, or the parent's own
	// standard fds when a stream is not redirected (the fd is -1).
	files := []uintptr{
		uintptr(fdOrStd(p2cread, 0)),
		uintptr(fdOrStd(c2pwrite, 1)),
		uintptr(fdOrStd(errwrite, 2)),
	}
	attr := &syscall.ProcAttr{
		Dir:   cwd,
		Env:   env,
		Files: files,
		Sys:   sys,
	}

	// Try each candidate executable in turn, the way subprocess's child walks the
	// PATH-expanded list; a missing candidate (ENOENT) falls through to the next,
	// and the last error is reported if none exec.
	var lastErr error
	for _, path := range execList {
		pid, serr := syscall.ForkExec(path, argv, attr)
		if serr == nil {
			return objects.NewInt(int64(pid)), nil
		}
		lastErr = serr
		if serr != syscall.ENOENT && serr != syscall.ENOTDIR {
			break
		}
	}
	return nil, posixStatErr(lastErr)
}

// fdOrStd maps a pipe fd to itself, or a -1 (stream not redirected) to the
// matching standard fd so the child inherits the parent's stream.
func fdOrStd(fd, std int) int {
	if fd < 0 {
		return std
	}
	return fd
}

// forkExecStrSeq reads a list or tuple of str/bytes into a []string, the form
// ForkExec wants for argv and the environment.
func forkExecStrSeq(name string, o objects.Object) ([]string, error) {
	if o == objects.None {
		return nil, nil
	}
	n, err := objects.Len(o)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "%s must be a sequence", name)
	}
	items, err := objects.Unpack(o, n)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(items))
	for i, it := range items {
		s, ok := posixFsPath(it)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "%s must contain only strings or bytes", name)
		}
		out[i] = s
	}
	return out, nil
}

// forkExecCredential builds the child's credential from the uid/gid/gids
// arguments, each of which is None to keep the parent's value. It returns nil
// when all three are None, so the child simply inherits.
func forkExecCredential(gidArg, gidsArg, uidArg objects.Object) (*syscall.Credential, error) {
	if gidArg == objects.None && gidsArg == objects.None && uidArg == objects.None {
		return nil, nil
	}
	cred := &syscall.Credential{
		Uid: uint32(syscall.Getuid()),
		Gid: uint32(syscall.Getgid()),
	}
	if uidArg != objects.None {
		v, ok := objects.AsIntValue(uidArg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "uid must be an integer")
		}
		cred.Uid = uint32(v)
	}
	if gidArg != objects.None {
		v, ok := objects.AsIntValue(gidArg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "gid must be an integer")
		}
		cred.Gid = uint32(v)
	}
	if gidsArg != objects.None {
		gids, err := objects.Len(gidsArg)
		if err != nil {
			return nil, objects.Raise(objects.TypeError, "gids must be a sequence")
		}
		items, err := objects.Unpack(gidsArg, gids)
		if err != nil {
			return nil, err
		}
		groups := make([]uint32, len(items))
		for i, it := range items {
			v, ok := objects.AsIntValue(it)
			if !ok {
				return nil, objects.Raise(objects.TypeError, "gids must contain only integers")
			}
			groups[i] = uint32(v)
		}
		cred.Groups = groups
	}
	return cred, nil
}
