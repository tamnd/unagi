package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestArrayUDeprecationBestEffort exercises the best-effort skip path of the 'u'
// type-code deprecation hook. A bare runtime unit test bundles only native
// modules, so the warnings floor module is absent; the hook must then skip
// silently and the array must still build. The warning-fires path, the message
// text and the error-promotion abort are covered end to end by the 2375
// conformance fixture, which runs through the full pipeline where warnings is
// present.
func TestArrayUDeprecationBestEffort(t *testing.T) {
	if _, err := ImportModule("warnings"); err == nil {
		t.Skip("warnings module present; best-effort skip path not exercised")
	}
	if err := arrayUDeprecationWarn(); err != nil {
		t.Fatalf("arrayUDeprecationWarn with no warnings module: %v", err)
	}
	// Construction still proceeds through the skipped hook and yields the 'u' array.
	a, err := objects.Call(arrayType, []objects.Object{objects.NewStr("u"), objects.NewStr("hi")})
	if err != nil {
		t.Fatalf("array('u', 'hi'): %v", err)
	}
	if got := objects.Repr(a); got != "array('u', 'hi')" {
		t.Fatalf("array('u', 'hi') = %s, want array('u', 'hi')", got)
	}
}
