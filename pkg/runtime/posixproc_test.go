//go:build !windows

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func posixAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("posix")
	if err != nil {
		t.Fatalf("import posix: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("posix.%s: %v", name, err)
	}
	return v
}

func callWaitInt(t *testing.T, fn objects.Object, arg int64) int64 {
	t.Helper()
	r, err := objects.Call(fn, []objects.Object{objects.NewInt(arg)})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	v, ok := objects.AsInt(r)
	if !ok {
		t.Fatalf("result %v is not an int", r)
	}
	return v
}

func callWaitBool(t *testing.T, fn objects.Object, arg int64) bool {
	t.Helper()
	r, err := objects.Call(fn, []objects.Object{objects.NewInt(arg)})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	b, ok := objects.AsBool(r)
	if !ok {
		t.Fatalf("result %v is not a bool", r)
	}
	return b
}

func TestPosixWaitStatusMacros(t *testing.T) {
	// A normal exit with code 42: bits 8+ hold the code, low 7 bits are clear.
	stExit := int64(42 << 8)
	if !callWaitBool(t, posixAttr(t, "WIFEXITED"), stExit) {
		t.Errorf("WIFEXITED(exit) = false")
	}
	if callWaitBool(t, posixAttr(t, "WIFSIGNALED"), stExit) || callWaitBool(t, posixAttr(t, "WIFSTOPPED"), stExit) {
		t.Errorf("exit status also reported as signaled/stopped")
	}
	if v := callWaitInt(t, posixAttr(t, "WEXITSTATUS"), stExit); v != 42 {
		t.Errorf("WEXITSTATUS(exit) = %d, want 42", v)
	}

	// Killed by signal 15, no core: the term signal is in the low 7 bits.
	stSig := int64(15)
	if !callWaitBool(t, posixAttr(t, "WIFSIGNALED"), stSig) || callWaitBool(t, posixAttr(t, "WIFEXITED"), stSig) {
		t.Errorf("WIFSIGNALED(15) misclassified")
	}
	if v := callWaitInt(t, posixAttr(t, "WTERMSIG"), stSig); v != 15 {
		t.Errorf("WTERMSIG(15) = %d, want 15", v)
	}
	if callWaitBool(t, posixAttr(t, "WCOREDUMP"), stSig) {
		t.Errorf("WCOREDUMP without the 0x80 bit is true")
	}
	if !callWaitBool(t, posixAttr(t, "WCOREDUMP"), stSig|0x80) {
		t.Errorf("WCOREDUMP with the 0x80 bit is false")
	}

	// Stopped: low byte is 0x7f. This is the case Go's darwin Stopped() gets
	// wrong for SIGSTOP, so it must come from the open-coded macro.
	stStop := int64((19 << 8) | 0x7f)
	if !callWaitBool(t, posixAttr(t, "WIFSTOPPED"), stStop) || callWaitBool(t, posixAttr(t, "WIFEXITED"), stStop) {
		t.Errorf("WIFSTOPPED misclassified")
	}
	if v := callWaitInt(t, posixAttr(t, "WSTOPSIG"), stStop); v != 19 {
		t.Errorf("WSTOPSIG = %d, want 19", v)
	}
}

func TestPosixWaitstatusToExitcode(t *testing.T) {
	f := posixAttr(t, "waitstatus_to_exitcode")
	if v := callWaitInt(t, f, 7<<8); v != 7 {
		t.Errorf("exit code 7 -> %d", v)
	}
	if v := callWaitInt(t, f, 9); v != -9 {
		t.Errorf("killed by 9 -> %d, want -9", v)
	}
	// A stopped status has no exit code and must raise ValueError.
	if _, err := objects.Call(f, []objects.Object{objects.NewInt((19 << 8) | 0x7f)}); err == nil {
		t.Errorf("waitstatus_to_exitcode(stopped) did not raise")
	}
}

func TestPosixInheritableRoundTrip(t *testing.T) {
	pipe := posixAttr(t, "pipe")
	setInh := posixAttr(t, "set_inheritable")
	getInh := posixAttr(t, "get_inheritable")
	closeFn := posixAttr(t, "close")

	pair, err := objects.Call(pipe, nil)
	if err != nil {
		t.Fatalf("pipe(): %v", err)
	}
	items, err := objects.Unpack(pair, 2)
	if err != nil {
		t.Fatalf("pipe returned %v (%v)", pair, err)
	}
	r, w := items[0], items[1]

	// os.pipe fds are non-inheritable, matching CPython.
	for _, fd := range []objects.Object{r, w} {
		got, err := objects.Call(getInh, []objects.Object{fd})
		if err != nil {
			t.Fatalf("get_inheritable: %v", err)
		}
		if b, _ := objects.AsBool(got); b {
			t.Errorf("fresh pipe fd is inheritable")
		}
	}
	// set_inheritable(True) then read back True, and back to False.
	if _, err := objects.Call(setInh, []objects.Object{w, objects.True}); err != nil {
		t.Fatalf("set_inheritable(True): %v", err)
	}
	got, _ := objects.Call(getInh, []objects.Object{w})
	if b, _ := objects.AsBool(got); !b {
		t.Errorf("fd not inheritable after set_inheritable(True)")
	}
	if _, err := objects.Call(setInh, []objects.Object{w, objects.False}); err != nil {
		t.Fatalf("set_inheritable(False): %v", err)
	}
	got, _ = objects.Call(getInh, []objects.Object{w})
	if b, _ := objects.AsBool(got); b {
		t.Errorf("fd inheritable after set_inheritable(False)")
	}
	_, _ = objects.Call(closeFn, []objects.Object{r})
	_, _ = objects.Call(closeFn, []objects.Object{w})
}
