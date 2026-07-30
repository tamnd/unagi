package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestSysIdentityAttrs(t *testing.T) {
	mo, err := ImportModule("sys")
	if err != nil {
		t.Fatalf("import sys: %v", err)
	}
	attr := func(name string) objects.Object {
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("sys.%s: %v", name, err)
		}
		return v
	}
	if n, _ := objects.AsInt(attr("maxsize")); n != 9223372036854775807 {
		t.Errorf("sys.maxsize = %v, want 9223372036854775807", objects.Str(attr("maxsize")))
	}
	if n, _ := objects.AsInt(attr("maxunicode")); n != 0x10FFFF {
		t.Errorf("sys.maxunicode = %v, want 1114111", objects.Str(attr("maxunicode")))
	}
	if n, _ := objects.AsInt(attr("hexversion")); n != 0x030E06F0 {
		t.Errorf("sys.hexversion = %#x, want 0x030e06f0", n)
	}
	if s, _ := objects.AsStr(attr("byteorder")); s != "little" {
		t.Errorf("sys.byteorder = %q, want little", s)
	}
	// platform reports the host, mapping Go's GOOS to CPython's spelling.
	if s, _ := objects.AsStr(attr("platform")); s != sysPlatform() {
		t.Errorf("sys.platform = %q, want %q", s, sysPlatform())
	}
	// version_info is a struct sequence whose triple gates the common version
	// check and whose named fields and repr match CPython.
	vi := attr("version_info")
	if got := objects.Repr(vi); got != "sys.version_info(major=3, minor=14, micro=6, releaselevel='final', serial=0)" {
		t.Errorf("sys.version_info = %s, want the sys.version_info(...) struct sequence repr", got)
	}
	if n, _ := objects.AsInt(func() objects.Object { v, _ := objects.LoadAttr(vi, "major"); return v }()); n != 3 {
		t.Errorf("sys.version_info.major = %d, want 3", n)
	}
	// implementation is a SimpleNamespace naming the interpreter cpython 3.14.
	im := attr("implementation")
	if s, _ := objects.AsStr(func() objects.Object { v, _ := objects.LoadAttr(im, "name"); return v }()); s != "cpython" {
		t.Errorf("sys.implementation.name = %q, want cpython", s)
	}
	if s, _ := objects.AsStr(func() objects.Object { v, _ := objects.LoadAttr(im, "cache_tag"); return v }()); s != "cpython-314" {
		t.Errorf("sys.implementation.cache_tag = %q, want cpython-314", s)
	}
}

// TestSysExit checks sys.exit raises SystemExit carrying the right code: None
// when omitted, the argument verbatim otherwise, and the arity TypeError for
// more than one argument.
func TestSysExit(t *testing.T) {
	codeOf := func(args []objects.Object) objects.Object {
		_, err := sysExit(args)
		exc, ok := err.(*objects.Exception)
		if !ok || exc.Kind != "SystemExit" {
			t.Fatalf("sysExit(%v) err = %v, want SystemExit", args, err)
		}
		code, cerr := objects.LoadAttr(exc, "code")
		if cerr != nil {
			t.Fatalf("SystemExit.code: %v", cerr)
		}
		return code
	}
	if c := codeOf(nil); c != objects.None {
		t.Errorf("exit() code = %v, want None", objects.Repr(c))
	}
	if c := codeOf([]objects.Object{objects.NewInt(3)}); objects.Str(c) != "3" {
		t.Errorf("exit(3) code = %v, want 3", objects.Repr(c))
	}
	_, err := sysExit([]objects.Object{objects.NewInt(1), objects.NewInt(2)})
	exc, ok := err.(*objects.Exception)
	if !ok || exc.Kind != "TypeError" {
		t.Fatalf("exit(1, 2) err = %v, want TypeError", err)
	}
}

func TestSysSwitchInterval(t *testing.T) {
	// The interval is process-global, so save and restore it around the test.
	switchIntervalMu.Lock()
	saved := switchInterval
	switchIntervalMu.Unlock()
	defer func() {
		switchIntervalMu.Lock()
		switchInterval = saved
		switchIntervalMu.Unlock()
	}()

	get := func() float64 {
		v, err := sysGetSwitchInterval(nil)
		if err != nil {
			t.Fatalf("getswitchinterval: %v", err)
		}
		f, _ := objects.AsFloat(v)
		return f
	}
	if got := get(); got != 0.005 {
		t.Errorf("default getswitchinterval = %v, want 0.005", got)
	}
	if _, err := sysSetSwitchInterval([]objects.Object{objects.NewFloat(0.01)}); err != nil {
		t.Fatalf("setswitchinterval(0.01): %v", err)
	}
	if got := get(); got != 0.01 {
		t.Errorf("after set, getswitchinterval = %v, want 0.01", got)
	}
	// An int coerces to a float.
	if _, err := sysSetSwitchInterval([]objects.Object{objects.NewInt(2)}); err != nil {
		t.Fatalf("setswitchinterval(2): %v", err)
	}
	if got := get(); got != 2 {
		t.Errorf("after set int, getswitchinterval = %v, want 2", got)
	}
	// Zero and negatives are the strictly-positive ValueError, and leave the
	// stored value untouched.
	for _, n := range []float64{0, -1.5} {
		_, err := sysSetSwitchInterval([]objects.Object{objects.NewFloat(n)})
		if err == nil || err.Error() != "ValueError: switch interval must be strictly positive" {
			t.Errorf("setswitchinterval(%v) error = %v, want strictly-positive ValueError", n, err)
		}
	}
	if got := get(); got != 2 {
		t.Errorf("after rejected sets, getswitchinterval = %v, want 2", got)
	}
	// A non-number is the real-number TypeError.
	_, err := sysSetSwitchInterval([]objects.Object{objects.NewStr("x")})
	if err == nil || err.Error() != "TypeError: must be real number, not str" {
		t.Errorf("setswitchinterval(\"x\") error = %v, want real-number TypeError", err)
	}
}

func TestSysRecursionLimit(t *testing.T) {
	// The limit is process-wide, so save and restore it around the test.
	saved := RecursionLimit()
	defer SetRecursionLimit(saved)

	get := func() int64 {
		v, err := sysGetRecursionLimit(nil)
		if err != nil {
			t.Fatalf("getrecursionlimit: %v", err)
		}
		n, _ := objects.AsInt(v)
		return n
	}
	if got := get(); got != 1000 {
		t.Errorf("default getrecursionlimit = %v, want 1000", got)
	}
	if _, err := sysSetRecursionLimit([]objects.Object{objects.NewInt(1500)}); err != nil {
		t.Fatalf("setrecursionlimit(1500): %v", err)
	}
	if got := get(); got != 1500 {
		t.Errorf("after set, getrecursionlimit = %v, want 1500", got)
	}
	// A limit below one is the strictly-positive ValueError and leaves the value.
	for _, n := range []int64{0, -5} {
		_, err := sysSetRecursionLimit([]objects.Object{objects.NewInt(n)})
		if err == nil || err.Error() != "ValueError: recursion limit must be greater or equal than 1" {
			t.Errorf("setrecursionlimit(%d) error = %v, want below-one ValueError", n, err)
		}
	}
	if got := get(); got != 1500 {
		t.Errorf("after rejected sets, getrecursionlimit = %v, want 1500", got)
	}
	// A non-integer is the integer-coercion TypeError.
	for _, bad := range []objects.Object{objects.NewStr("x"), objects.NewFloat(1.5), objects.None} {
		_, err := sysSetRecursionLimit([]objects.Object{bad})
		want := "TypeError: '" + bad.TypeName() + "' object cannot be interpreted as an integer"
		if err == nil || err.Error() != want {
			t.Errorf("setrecursionlimit(%s) error = %v, want %q", objects.Repr(bad), err, want)
		}
	}
}

// TestSysGetFrame drives sys._getframe over a thread's shadow stack: the default
// depth is the running frame, a depth walks to an ancestor, a depth past the
// bottom is the ValueError, and a non-integer depth is the coercion TypeError.
func TestSysGetFrame(t *testing.T) {
	th := objects.NewThread("t", false)
	PushFrame(th, "t.py", "<module>", "<module>", 1, false)
	PushFrame(th, "t.py", "outer", "outer", 10, true)
	PushFrame(th, "t.py", "inner", "inner", 20, true)
	defer func() {
		th.PopFrame()
		th.PopFrame()
		th.PopFrame()
	}()

	coName := func(f objects.Object) string {
		code, err := objects.LoadAttr(f, "f_code")
		if err != nil {
			t.Fatalf("f_code: %v", err)
		}
		name, err := objects.LoadAttr(code, "co_name")
		if err != nil {
			t.Fatalf("co_name: %v", err)
		}
		s, _ := objects.AsStr(name)
		return s
	}

	f, err := sysGetFrame(th, nil)
	if err != nil {
		t.Fatalf("_getframe(): %v", err)
	}
	if got := coName(f); got != "inner" {
		t.Errorf("_getframe() co_name = %q, want inner", got)
	}
	f, err = sysGetFrame(th, []objects.Object{objects.NewInt(2)})
	if err != nil {
		t.Fatalf("_getframe(2): %v", err)
	}
	if got := coName(f); got != "<module>" {
		t.Errorf("_getframe(2) co_name = %q, want <module>", got)
	}
	// A depth past the bottom of the stack is the ValueError.
	_, err = sysGetFrame(th, []objects.Object{objects.NewInt(50)})
	if err == nil || err.Error() != "ValueError: call stack is not deep enough" {
		t.Errorf("_getframe(50) error = %v, want too-deep ValueError", err)
	}
	// A non-integer depth is the integer-coercion TypeError.
	_, err = sysGetFrame(th, []objects.Object{objects.NewStr("x")})
	if err == nil || err.Error() != "TypeError: 'str' object cannot be interpreted as an integer" {
		t.Errorf("_getframe('x') error = %v, want coercion TypeError", err)
	}
}
