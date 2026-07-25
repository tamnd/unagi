package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestSysconfigDataSynth checks a _sysconfigdata_-prefixed import resolves to a
// synthesized module carrying an empty build_time_vars dict, regardless of the
// platform-specific suffix sysconfig computes.
func TestSysconfigDataSynth(t *testing.T) {
	for _, name := range []string{"_sysconfigdata__darwin_darwin", "_sysconfigdata__linux_x86_64-linux-gnu"} {
		mo, err := ImportModule(name)
		if err != nil {
			t.Fatalf("import %s: %v", name, err)
		}
		v, err := objects.LoadAttr(mo, "build_time_vars")
		if err != nil {
			t.Fatalf("%s.build_time_vars: %v", name, err)
		}
		if v.TypeName() != "dict" {
			t.Errorf("%s.build_time_vars is %s, want dict", name, v.TypeName())
		}
		n, err := objects.Len(v)
		if err != nil {
			t.Fatalf("len(build_time_vars): %v", err)
		}
		if n != 0 {
			t.Errorf("%s.build_time_vars has %d entries, want 0 (synthesized stand-in)", name, n)
		}
	}
}
