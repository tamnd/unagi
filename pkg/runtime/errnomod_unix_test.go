//go:build !windows

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// On POSIX hosts EWOULDBLOCK aliases EAGAIN, and errorcode keeps the canonical
// EAGAIN for that shared number. Windows keeps them distinct (EAGAIN is 11,
// EWOULDBLOCK is the winsock 10035), so this alias check is unix-only; the
// Windows values are pinned in errnomod_windows_test.go.
func TestErrnoEagainAliasUnix(t *testing.T) {
	mo, err := ImportModule("errno")
	if err != nil {
		t.Fatalf("import errno: %v", err)
	}
	if errnoAttrInt(t, mo, "EWOULDBLOCK") != errnoAttrInt(t, mo, "EAGAIN") {
		t.Fatal("EWOULDBLOCK should equal EAGAIN on POSIX")
	}
	code, err := objects.LoadAttr(mo, "errorcode")
	if err != nil {
		t.Fatalf("errno.errorcode: %v", err)
	}
	agn := errnoAttrInt(t, mo, "EAGAIN")
	v, err := objects.GetItem(code, objects.NewInt(agn))
	if err != nil {
		t.Fatalf("errorcode[EAGAIN]: %v", err)
	}
	if s, ok := objects.AsStr(v); !ok || s != "EAGAIN" {
		t.Fatalf("errorcode[EAGAIN] = %v, want \"EAGAIN\"", v)
	}
}
