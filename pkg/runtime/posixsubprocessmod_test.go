//go:build !windows

package runtime

import (
	"syscall"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestForkExecPipe spawns /bin/echo with its stdout wired to a pipe and checks
// the child both runs and writes the expected bytes, exercising the full
// fork_exec fd-plumbing path.
func TestForkExecPipe(t *testing.T) {
	sp, err := ImportModule("_posixsubprocess")
	if err != nil {
		t.Fatalf("import _posixsubprocess: %v", err)
	}
	fn, err := objects.LoadAttr(sp, "fork_exec")
	if err != nil {
		t.Fatalf("fork_exec attr: %v", err)
	}

	var fds [2]int
	if err := syscall.Pipe(fds[:]); err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = syscall.Close(fds[0]) }()

	argv := objects.NewList([]objects.Object{objects.NewStr("/bin/echo"), objects.NewStr("hi")})
	execList := objects.NewList([]objects.Object{objects.NewStr("/bin/echo")})
	none := objects.None
	// Match the 22-arg fork_exec signature: only argv(0), exec_list(1),
	// c2pwrite(9) and the pass-through flags matter here; the rest are None/-1.
	args := []objects.Object{
		argv, execList, objects.True, objects.NewTuple(nil), none, none,
		objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(int64(fds[1])),
		objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(-1),
		objects.True, objects.False, objects.NewInt(-1),
		none, none, none, objects.NewInt(-1), none,
	}
	pidObj, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("fork_exec: %v", err)
	}
	_ = syscall.Close(fds[1]) // parent drops the write end so the read sees EOF

	pid, ok := objects.AsInt(pidObj)
	if !ok || pid <= 0 {
		t.Fatalf("fork_exec returned %v, want a positive pid", pidObj)
	}

	buf := make([]byte, 16)
	n, _ := syscall.Read(fds[0], buf)
	if got := string(buf[:n]); got != "hi\n" {
		t.Errorf("child wrote %q, want %q", got, "hi\n")
	}
	var ws syscall.WaitStatus
	_, _ = syscall.Wait4(int(pid), &ws, 0, nil)
}

// TestForkExecMissing checks a nonexistent executable surfaces as an OSError
// (ENOENT), the error subprocess maps to FileNotFoundError.
func TestForkExecMissing(t *testing.T) {
	sp, _ := ImportModule("_posixsubprocess")
	fn, _ := objects.LoadAttr(sp, "fork_exec")
	argv := objects.NewList([]objects.Object{objects.NewStr("/no/such/bin_xyz")})
	execList := objects.NewList([]objects.Object{objects.NewStr("/no/such/bin_xyz")})
	none := objects.None
	args := []objects.Object{
		argv, execList, objects.True, objects.NewTuple(nil), none, none,
		objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(-1),
		objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(-1), objects.NewInt(-1),
		objects.True, objects.False, objects.NewInt(-1),
		none, none, none, objects.NewInt(-1), none,
	}
	if _, err := objects.Call(fn, args); err == nil {
		t.Errorf("fork_exec on a missing executable did not raise")
	}
}

// TestPosixKillSig0 uses os.kill with signal 0, the existence probe that sends
// no signal, against the test process itself.
func TestPosixKillSig0(t *testing.T) {
	kill := posixAttr(t, "kill")
	self := int64(syscall.Getpid())
	if _, err := objects.Call(kill, []objects.Object{objects.NewInt(self), objects.NewInt(0)}); err != nil {
		t.Errorf("kill(self, 0): %v", err)
	}
}
