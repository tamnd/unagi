//go:build windows

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// raisedKind reports whether err is a raised Python exception of the given kind.
func raisedKind(err error, kind string) bool {
	e, ok := err.(*objects.Exception)
	return ok && e.Kind == kind
}

func winSelectAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("select")
	if err != nil {
		t.Fatalf("import select: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("select.%s: %v", name, err)
	}
	return v
}

// select on Windows ships only select() and the error alias; CPython has no
// poll/epoll there, which is what makes selectors fall back to SelectSelector.
func TestSelectWindowsSurface(t *testing.T) {
	mo, err := ImportModule("select")
	if err != nil {
		t.Fatalf("import select: %v", err)
	}
	if _, err := objects.LoadAttr(mo, "select"); err != nil {
		t.Errorf("select.select missing: %v", err)
	}
	errAttr, err := objects.LoadAttr(mo, "error")
	if err != nil {
		t.Fatalf("select.error missing: %v", err)
	}
	oserr, ok := objects.ExcClassValue("OSError")
	if !ok {
		t.Fatal("OSError class missing")
	}
	if errAttr != oserr {
		t.Errorf("select.error is not OSError")
	}
	if _, err := objects.LoadAttr(mo, "poll"); err == nil {
		t.Errorf("select.poll should be absent on Windows")
	}
}

// A list of more than FD_SETSIZE (512) sockets raises ValueError before select
// is ever called, exactly as CPython's seq2set does on Windows.
func TestSelectWindowsTooManyFds(t *testing.T) {
	sel := winSelectAttr(t, "select")
	big := make([]objects.Object, 0, 513)
	for i := 0; i < 513; i++ {
		big = append(big, objects.NewInt(int64(i)))
	}
	_, err := objects.Call(sel, []objects.Object{objects.NewList(big), objects.NewList(nil), objects.NewList(nil), objects.NewFloat(0)})
	if err == nil {
		t.Fatal("select with 513 fds should raise")
	}
	if !raisedKind(err, "ValueError") {
		t.Errorf("want ValueError, got %v", err)
	}
	// 512 sockets is within the bound, so it clears the ValueError check and
	// reaches winsock instead (which fails for other reasons, not ValueError).
	ok512 := make([]objects.Object, 0, 512)
	for i := 0; i < 512; i++ {
		ok512 = append(ok512, objects.NewInt(int64(i)))
	}
	_, err = objects.Call(sel, []objects.Object{objects.NewList(ok512), objects.NewList(nil), objects.NewList(nil), objects.NewFloat(0)})
	if raisedKind(err, "ValueError") {
		t.Errorf("512 fds should not raise ValueError, got %v", err)
	}
}

// With winsock uninitialised (the socket module has not run WSAStartup), a call
// with empty lists reports an OSError, matching CPython's select in the same
// state. This exercises the ws2_32 select() path and the OSError mapping.
func TestSelectWindowsEmptyRaisesOSError(t *testing.T) {
	sel := winSelectAttr(t, "select")
	empty := objects.NewList(nil)
	_, err := objects.Call(sel, []objects.Object{empty, empty, empty, objects.NewFloat(0)})
	if err == nil {
		t.Fatal("select([],[],[],0) should raise on Windows")
	}
	if !raisedKind(err, "OSError") {
		t.Errorf("want OSError, got %v", err)
	}
}

// A non-int without fileno() is a TypeError, the same message every host gives.
func TestSelectWindowsBadArg(t *testing.T) {
	sel := winSelectAttr(t, "select")
	bad := objects.NewList([]objects.Object{objects.NewStr("x")})
	empty := objects.NewList(nil)
	_, err := objects.Call(sel, []objects.Object{bad, empty, empty, objects.NewFloat(0)})
	if !raisedKind(err, "TypeError") {
		t.Errorf("want TypeError, got %v", err)
	}
}
