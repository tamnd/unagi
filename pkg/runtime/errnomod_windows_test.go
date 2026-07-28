//go:build windows

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// The Windows errno table is pinned to the values CPython 3.14.6 reports on
// windows/amd64: the socket names live in the 10000 winsock range, EAGAIN and
// EWOULDBLOCK are distinct, and both the E* and WSAE* name sets are present.
func TestErrnoWindowsValues(t *testing.T) {
	mo, err := ImportModule("errno")
	if err != nil {
		t.Fatalf("import errno: %v", err)
	}
	want := map[string]int64{
		"EAGAIN":         11,
		"EWOULDBLOCK":    10035,
		"WSAEWOULDBLOCK": 10035,
		"EADDRINUSE":     10048,
		"WSAEADDRINUSE":  10048,
		"EDEADLK":        36,
		"EDEADLOCK":      36,
		"WSABASEERR":     10000,
	}
	for name, n := range want {
		if got := errnoAttrInt(t, mo, name); got != n {
			t.Fatalf("errno.%s = %d, want %d", name, got, n)
		}
	}
	if errnoAttrInt(t, mo, "EAGAIN") == errnoAttrInt(t, mo, "EWOULDBLOCK") {
		t.Fatal("EAGAIN and EWOULDBLOCK are distinct on Windows")
	}

	code, err := objects.LoadAttr(mo, "errorcode")
	if err != nil {
		t.Fatalf("errno.errorcode: %v", err)
	}
	// For the shared numbers the winsock name wins, and EDEADLOCK wins 36.
	canon := map[int64]string{
		36:    "EDEADLOCK",
		10048: "WSAEADDRINUSE",
		10035: "WSAEWOULDBLOCK",
	}
	for num, name := range canon {
		v, err := objects.GetItem(code, objects.NewInt(num))
		if err != nil {
			t.Fatalf("errorcode[%d]: %v", num, err)
		}
		if s, ok := objects.AsStr(v); !ok || s != name {
			t.Fatalf("errorcode[%d] = %v, want %q", num, v, name)
		}
	}
	// errorcode has one entry per distinct number: 101 on Windows.
	n, err := objects.Len(code)
	if err != nil {
		t.Fatalf("len(errorcode): %v", err)
	}
	if n != 101 {
		t.Fatalf("len(errorcode) = %d, want 101", n)
	}
}
