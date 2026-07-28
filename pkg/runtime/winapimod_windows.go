//go:build windows

package runtime

import (
	"syscall"
	"unsafe"

	"github.com/tamnd/unagi/pkg/objects"
)

// _winapi is the C accelerator subprocess.py runs on Windows, the counterpart of
// _posixsubprocess on unix: where posix forks and execs, Windows spawns a child
// with CreateProcess, its stdio wired to pipe handles. `import subprocess` does a
// hard `from _winapi import (CREATE_NEW_CONSOLE, ...)` of twenty creation and
// startup constants at module scope, and a few functions are bound as default
// arguments at class-definition time (Popen.Handle.Close takes CloseHandle,
// _internal_poll takes WaitForSingleObject/WAIT_OBJECT_0/GetExitCodeProcess), so
// all of those have to exist for the import to complete. shutil then imports
// _winapi under a try/except, so registering the module is also what unblocks
// `import tempfile` (tempfile -> shutil -> _winapi).
//
// This carries the surface subprocess.Popen._execute_child drives on Windows:
// CreatePipe/GetStdHandle/DuplicateHandle for the stdio plumbing, CreateProcess
// to launch, WaitForSingleObject/GetExitCodeProcess/TerminateProcess for the
// wait-and-reap, GetFileType/GetCurrentProcess/CloseHandle for the bookkeeping,
// and NeedCurrentDirectoryForExePath for shutil.which. Handles are kernel HANDLEs
// widened to int, the same model nt uses for fds (ntfd_windows.go).
//
// DIVERGENCES: CopyFile2 is not exposed, so shutil.copyfile takes its portable
// read/write fallback rather than the CopyFileW fast path. The STARTUPINFOEX
// lpAttributeList handle_list is not honored: CreateProcess inherits every
// inheritable handle when inherit is set, which is the same set subprocess makes
// inheritable by hand (the CreatePipe ends are non-inheritable until it dups
// them), so the standard Popen path inherits exactly the three std handles it
// intends to.

func init() {
	moduleTable["_winapi"] = &moduleEntry{builtin: true, exec: initWinapi}
}

// winapiConsts are the creation, startup, wait and file-type values CPython's
// _winapi reports on Windows, pinned to the 3.14.6 oracle on windows/amd64. The
// STD_*_HANDLE values are the unsigned DWORD forms (STD_INPUT_HANDLE is
// 0xFFFFFFF6, not -10); GetStdHandle narrows them back to the signed id.
var winapiConsts = []struct {
	name string
	val  int64
}{
	{"CREATE_NEW_CONSOLE", 16},
	{"CREATE_NEW_PROCESS_GROUP", 512},
	{"CREATE_NO_WINDOW", 134217728},
	{"DETACHED_PROCESS", 8},
	{"CREATE_DEFAULT_ERROR_MODE", 67108864},
	{"CREATE_BREAKAWAY_FROM_JOB", 16777216},
	{"ABOVE_NORMAL_PRIORITY_CLASS", 32768},
	{"BELOW_NORMAL_PRIORITY_CLASS", 16384},
	{"HIGH_PRIORITY_CLASS", 128},
	{"IDLE_PRIORITY_CLASS", 64},
	{"NORMAL_PRIORITY_CLASS", 32},
	{"REALTIME_PRIORITY_CLASS", 256},
	{"STD_INPUT_HANDLE", 4294967286},
	{"STD_OUTPUT_HANDLE", 4294967285},
	{"STD_ERROR_HANDLE", 4294967284},
	{"SW_HIDE", 0},
	{"STARTF_USESTDHANDLES", 256},
	{"STARTF_USESHOWWINDOW", 1},
	{"STARTF_FORCEONFEEDBACK", 64},
	{"STARTF_FORCEOFFFEEDBACK", 128},
	{"DUPLICATE_SAME_ACCESS", 2},
	{"DUPLICATE_CLOSE_SOURCE", 1},
	{"FILE_TYPE_UNKNOWN", 0},
	{"FILE_TYPE_DISK", 1},
	{"FILE_TYPE_CHAR", 2},
	{"FILE_TYPE_PIPE", 3},
	{"FILE_TYPE_REMOTE", 32768},
	{"INFINITE", 4294967295},
	{"WAIT_OBJECT_0", 0},
	{"WAIT_ABANDONED_0", 128},
	{"WAIT_TIMEOUT", 258},
	{"STILL_ACTIVE", 259},
	{"PROCESS_ALL_ACCESS", 2097151},
	{"PROCESS_DUP_HANDLE", 64},
	{"ERROR_PRIVILEGE_NOT_HELD", 1314},
	{"ERROR_ACCESS_DENIED", 5},
}

const (
	winStartfUseStdHandles  = 256
	winStartfUseShowWindow  = 1
	winCreateUnicodeEnviron = 0x00000400 // CREATE_UNICODE_ENVIRONMENT
)

var (
	winKernel32       = syscall.NewLazyDLL("kernel32.dll")
	winProcNeedCurDir = winKernel32.NewProc("NeedCurrentDirectoryForExePathW")
)

func initWinapi(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	for _, c := range winapiConsts {
		if err := set(c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}
	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"CreatePipe", winCreatePipe},
		{"GetStdHandle", winGetStdHandle},
		{"GetCurrentProcess", winGetCurrentProcess},
		{"DuplicateHandle", winDuplicateHandle},
		{"CloseHandle", winCloseHandle},
		{"GetFileType", winGetFileType},
		{"CreateProcess", winCreateProcess},
		{"WaitForSingleObject", winWaitForSingleObject},
		{"GetExitCodeProcess", winGetExitCodeProcess},
		{"TerminateProcess", winTerminateProcess},
		{"NeedCurrentDirectoryForExePath", winNeedCurrentDirectoryForExePath},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// winArgInt reads one integer argument at position i, raising the CPython
// not-an-integer TypeError otherwise.
func winArgInt(name string, args []objects.Object, i int) (int64, error) {
	v, ok := objects.AsInt(args[i])
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[i].TypeName())
	}
	return v, nil
}

// winHandle reads a HANDLE argument, the int a handle was widened into. A
// pseudo-handle such as GetCurrentProcess()'s -1 round-trips through int64.
func winHandle(name string, args []objects.Object, i int) (syscall.Handle, error) {
	v, err := winArgInt(name, args, i)
	if err != nil {
		return 0, err
	}
	return syscall.Handle(uintptr(v)), nil
}

// winCreatePipe creates an anonymous pipe and returns its (read, write) handle
// ends. The handles are non-inheritable, matching CPython; subprocess makes the
// child's end inheritable itself with DuplicateHandle. The pipe_attrs argument is
// accepted and ignored, as it is in _winapi.
func winCreatePipe(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "CreatePipe() takes exactly 2 arguments (%d given)", len(args))
	}
	size, err := winArgInt("CreatePipe", args, 1)
	if err != nil {
		return nil, err
	}
	var r, w syscall.Handle
	if serr := syscall.CreatePipe(&r, &w, nil, uint32(size)); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewTuple([]objects.Object{
		objects.NewInt(int64(r)),
		objects.NewInt(int64(w)),
	}), nil
}

// winGetStdHandle returns the process's standard handle for the given id
// (STD_INPUT_HANDLE etc., the unsigned DWORD form), narrowed to the signed value
// GetStdHandle wants.
func winGetStdHandle(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "GetStdHandle() takes exactly 1 argument (%d given)", len(args))
	}
	id, err := winArgInt("GetStdHandle", args, 0)
	if err != nil {
		return nil, err
	}
	h, serr := syscall.GetStdHandle(int(int32(id)))
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(h)), nil
}

// winGetCurrentProcess returns the pseudo-handle for the current process, the
// value DuplicateHandle takes as both source and target process.
func winGetCurrentProcess(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "GetCurrentProcess() takes no arguments (%d given)", len(args))
	}
	h, serr := syscall.GetCurrentProcess()
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(h)), nil
}

// winDuplicateHandle duplicates a handle across processes, the call subprocess
// uses to make a pipe end inheritable. It mirrors the six-argument Win32 shape:
// source process, source handle, target process, desired access, inherit flag,
// options.
func winDuplicateHandle(args []objects.Object) (objects.Object, error) {
	if len(args) < 6 || len(args) > 7 {
		return nil, objects.Raise(objects.TypeError, "DuplicateHandle() takes 6 or 7 arguments (%d given)", len(args))
	}
	srcProc, err := winHandle("DuplicateHandle", args, 0)
	if err != nil {
		return nil, err
	}
	src, err := winHandle("DuplicateHandle", args, 1)
	if err != nil {
		return nil, err
	}
	tgtProc, err := winHandle("DuplicateHandle", args, 2)
	if err != nil {
		return nil, err
	}
	access, err := winArgInt("DuplicateHandle", args, 3)
	if err != nil {
		return nil, err
	}
	inherit, err := objects.TruthOf(args[4])
	if err != nil {
		return nil, err
	}
	options, err := winArgInt("DuplicateHandle", args, 5)
	if err != nil {
		return nil, err
	}
	var target syscall.Handle
	if serr := syscall.DuplicateHandle(srcProc, src, tgtProc, &target, uint32(access), inherit, uint32(options)); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(target)), nil
}

// winCloseHandle closes a handle and returns None.
func winCloseHandle(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "CloseHandle() takes exactly 1 argument (%d given)", len(args))
	}
	h, err := winHandle("CloseHandle", args, 0)
	if err != nil {
		return nil, err
	}
	if serr := syscall.CloseHandle(h); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.None, nil
}

// winGetFileType reports whether a handle is a character device, disk file, pipe
// or unknown, the query subprocess uses to skip console handles when filtering
// its inherit list.
func winGetFileType(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "GetFileType() takes exactly 1 argument (%d given)", len(args))
	}
	h, err := winHandle("GetFileType", args, 0)
	if err != nil {
		return nil, err
	}
	t, serr := syscall.GetFileType(h)
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(t)), nil
}

// winWaitForSingleObject blocks until the handle is signaled or the millisecond
// timeout elapses (INFINITE is 0xFFFFFFFF), returning the wait result code.
func winWaitForSingleObject(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "WaitForSingleObject() takes exactly 2 arguments (%d given)", len(args))
	}
	h, err := winHandle("WaitForSingleObject", args, 0)
	if err != nil {
		return nil, err
	}
	ms, err := winArgInt("WaitForSingleObject", args, 1)
	if err != nil {
		return nil, err
	}
	event, serr := syscall.WaitForSingleObject(h, uint32(ms))
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(event)), nil
}

// winGetExitCodeProcess returns a child's exit code, or STILL_ACTIVE (259) while
// it is still running.
func winGetExitCodeProcess(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "GetExitCodeProcess() takes exactly 1 argument (%d given)", len(args))
	}
	h, err := winHandle("GetExitCodeProcess", args, 0)
	if err != nil {
		return nil, err
	}
	var code uint32
	if serr := syscall.GetExitCodeProcess(h, &code); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(code)), nil
}

// winTerminateProcess forces a child to exit with the given code and returns None.
func winTerminateProcess(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "TerminateProcess() takes exactly 2 arguments (%d given)", len(args))
	}
	h, err := winHandle("TerminateProcess", args, 0)
	if err != nil {
		return nil, err
	}
	code, err := winArgInt("TerminateProcess", args, 1)
	if err != nil {
		return nil, err
	}
	if serr := syscall.TerminateProcess(h, uint32(code)); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.None, nil
}

// winNeedCurrentDirectoryForExePath answers whether Windows would search the
// current directory when resolving an unqualified executable name, the guard
// shutil.which uses before it prepends the cwd.
func winNeedCurrentDirectoryForExePath(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "NeedCurrentDirectoryForExePath() takes exactly 1 argument (%d given)", len(args))
	}
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "argument must be str, not %s", args[0].TypeName())
	}
	p16, serr := syscall.UTF16PtrFromString(name)
	if serr != nil {
		return nil, objects.Raise(objects.ValueError, "%s", serr.Error())
	}
	r, _, _ := winProcNeedCurDir.Call(uintptr(unsafe.Pointer(p16)))
	return objects.NewBool(r != 0), nil
}

// winCreateProcess launches a child and returns (hProcess, hThread, pid, tid).
// It mirrors the nine-argument _winapi shape subprocess calls: application name,
// command line, the two ignored security-attribute slots, the inherit-handles
// flag, creation flags, an environment mapping (or None to inherit), a working
// directory (or None), and a STARTUPINFO object whose dwFlags/std-handle/show
// fields are read back onto the Win32 STARTUPINFO.
func winCreateProcess(args []objects.Object) (objects.Object, error) {
	if len(args) != 9 {
		return nil, objects.Raise(objects.TypeError, "CreateProcess() takes exactly 9 arguments (%d given)", len(args))
	}

	appPtr, err := winOptStrPtr("CreateProcess", args[0])
	if err != nil {
		return nil, err
	}
	cmdPtr, err := winOptStrPtr("CreateProcess", args[1])
	if err != nil {
		return nil, err
	}
	inherit, err := objects.TruthOf(args[4])
	if err != nil {
		return nil, err
	}
	flags, err := winArgInt("CreateProcess", args, 5)
	if err != nil {
		return nil, err
	}
	cwdPtr, err := winOptStrPtr("CreateProcess", args[7])
	if err != nil {
		return nil, err
	}

	creationFlags := uint32(flags)
	var envPtr *uint16
	if args[6] != objects.None {
		block, berr := winEnvBlock(args[6])
		if berr != nil {
			return nil, berr
		}
		envPtr = &block[0]
		// A Unicode environment block must be flagged as such, the way _winapi
		// does when it builds the block from a mapping.
		creationFlags |= winCreateUnicodeEnviron
	}

	var startup syscall.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	if err := winFillStartupInfo(&startup, args[8]); err != nil {
		return nil, err
	}

	var pi syscall.ProcessInformation
	if serr := syscall.CreateProcess(appPtr, cmdPtr, nil, nil, inherit, creationFlags, envPtr, cwdPtr, &startup, &pi); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewTuple([]objects.Object{
		objects.NewInt(int64(pi.Process)),
		objects.NewInt(int64(pi.Thread)),
		objects.NewInt(int64(pi.ProcessId)),
		objects.NewInt(int64(pi.ThreadId)),
	}), nil
}

// winOptStrPtr converts a str-or-None argument to a UTF-16 pointer, returning nil
// for None so CreateProcess sees NULL (inherit / parse-from-command-line).
func winOptStrPtr(name string, o objects.Object) (*uint16, error) {
	if o == objects.None {
		return nil, nil
	}
	s, ok := objects.AsStr(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "%s: expected str or None, not %s", name, o.TypeName())
	}
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil, objects.Raise(objects.ValueError, "%s", err.Error())
	}
	return p, nil
}

// winFillStartupInfo copies the fields CreateProcess honors off the Python
// STARTUPINFO object: the flags, and, when the matching flag bit is set, the
// three std handles and the show-window value.
func winFillStartupInfo(si *syscall.StartupInfo, o objects.Object) error {
	flags, err := winAttrInt(o, "dwFlags")
	if err != nil {
		return err
	}
	si.Flags = uint32(flags)
	if flags&winStartfUseStdHandles != 0 {
		in, err := winAttrHandle(o, "hStdInput")
		if err != nil {
			return err
		}
		out, err := winAttrHandle(o, "hStdOutput")
		if err != nil {
			return err
		}
		errh, err := winAttrHandle(o, "hStdError")
		if err != nil {
			return err
		}
		si.StdInput, si.StdOutput, si.StdErr = in, out, errh
	}
	if flags&winStartfUseShowWindow != 0 {
		show, err := winAttrInt(o, "wShowWindow")
		if err != nil {
			return err
		}
		si.ShowWindow = uint16(show)
	}
	return nil
}

// winAttrInt reads an integer attribute off an object.
func winAttrInt(o objects.Object, name string) (int64, error) {
	v, err := objects.LoadAttr(o, name)
	if err != nil {
		return 0, err
	}
	n, ok := objects.AsInt(v)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "startupinfo.%s must be an integer", name)
	}
	return n, nil
}

// winAttrHandle reads a handle-valued attribute off an object.
func winAttrHandle(o objects.Object, name string) (syscall.Handle, error) {
	n, err := winAttrInt(o, name)
	if err != nil {
		return 0, err
	}
	return syscall.Handle(uintptr(n)), nil
}

// winEnvBlock builds the double-null-terminated UTF-16 environment block Win32
// wants from a str->str mapping, one "KEY=VALUE\0" run per entry followed by the
// final terminator. An empty mapping yields the two-null empty block.
func winEnvBlock(env objects.Object) ([]uint16, error) {
	it, err := objects.Iter(env)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "environment must be a mapping")
	}
	var buf []uint16
	for {
		k, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		v, err := objects.GetItem(env, k)
		if err != nil {
			return nil, err
		}
		ks, ok := objects.AsStr(k)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "environment keys must be str")
		}
		vs, ok := objects.AsStr(v)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "environment values must be str")
		}
		enc, err := syscall.UTF16FromString(ks + "=" + vs)
		if err != nil {
			return nil, objects.Raise(objects.ValueError, "%s", err.Error())
		}
		buf = append(buf, enc...) // enc already ends with the per-string NUL
	}
	buf = append(buf, 0) // block terminator
	if len(buf) == 1 {
		buf = append(buf, 0) // empty environment is "\0\0"
	}
	return buf, nil
}
