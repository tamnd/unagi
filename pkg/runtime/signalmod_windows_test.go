//go:build windows

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func winSignalAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("_signal")
	if err != nil {
		t.Fatalf("import _signal: %v", err)
	}
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_signal.%s: %v", name, err)
	}
	return v
}

func winSignalInt(t *testing.T, name string) int64 {
	t.Helper()
	v, ok := objects.AsInt(winSignalAttr(t, name))
	if !ok {
		t.Fatalf("_signal.%s is not an int", name)
	}
	return v
}

func TestSignalWindowsConstants(t *testing.T) {
	want := map[string]int64{
		"SIG_DFL": 0, "SIG_IGN": 1,
		"SIGINT": 2, "SIGILL": 4, "SIGFPE": 8, "SIGSEGV": 11,
		"SIGTERM": 15, "SIGBREAK": 21, "SIGABRT": 22,
		"CTRL_C_EVENT": 0, "CTRL_BREAK_EVENT": 1,
		"NSIG": 23,
	}
	for name, n := range want {
		if got := winSignalInt(t, name); got != n {
			t.Errorf("_signal.%s = %d, want %d", name, got, n)
		}
	}
}

func TestSignalWindowsGetsignalDefaults(t *testing.T) {
	getsignal := winSignalAttr(t, "getsignal")
	// SIGINT is seeded with default_int_handler, matching CPython's startup.
	h, err := objects.Call(getsignal, []objects.Object{winSignalAttr(t, "SIGINT")})
	if err != nil {
		t.Fatalf("getsignal(SIGINT): %v", err)
	}
	name, err := objects.LoadAttr(h, "__name__")
	if err != nil {
		t.Fatalf("handler __name__: %v", err)
	}
	if s, _ := objects.AsStr(name); s != "default_int_handler" {
		t.Errorf("getsignal(SIGINT) = %v, want default_int_handler", h)
	}
	// SIGTERM has nothing installed, so it reports SIG_DFL.
	d, err := objects.Call(getsignal, []objects.Object{winSignalAttr(t, "SIGTERM")})
	if err != nil {
		t.Fatalf("getsignal(SIGTERM): %v", err)
	}
	if v, ok := objects.AsInt(d); !ok || v != sigDfl {
		t.Errorf("getsignal(SIGTERM) = %v, want SIG_DFL", d)
	}
}

func TestSignalWindowsInstallRoundTrip(t *testing.T) {
	sig := winSignalAttr(t, "signal")
	getsignal := winSignalAttr(t, "getsignal")
	sigabrt := winSignalAttr(t, "SIGABRT")
	handler := objects.NewFunc("h", -1, func(args []objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	prev, err := objects.Call(sig, []objects.Object{sigabrt, handler})
	if err != nil {
		t.Fatalf("signal(SIGABRT, handler): %v", err)
	}
	if v, ok := objects.AsInt(prev); !ok || v != sigDfl {
		t.Errorf("previous handler = %v, want SIG_DFL", prev)
	}
	cur, err := objects.Call(getsignal, []objects.Object{sigabrt})
	if err != nil {
		t.Fatalf("getsignal: %v", err)
	}
	if cur != handler {
		t.Errorf("getsignal(SIGABRT) = %v, want the installed handler", cur)
	}
	old, err := objects.Call(sig, []objects.Object{sigabrt, winSignalAttr(t, "SIG_DFL")})
	if err != nil {
		t.Fatalf("restore SIG_DFL: %v", err)
	}
	if old != handler {
		t.Errorf("restore returned %v, want the installed handler", old)
	}
}

func TestSignalWindowsStrsignalAndValid(t *testing.T) {
	strsignal := winSignalAttr(t, "strsignal")
	s, err := objects.Call(strsignal, []objects.Object{winSignalAttr(t, "SIGINT")})
	if err != nil {
		t.Fatalf("strsignal(SIGINT): %v", err)
	}
	if text, ok := objects.AsStr(s); !ok || text != "Interrupt" {
		t.Errorf("strsignal(SIGINT) = %v, want \"Interrupt\"", s)
	}
	// SIGBREAK is a valid signal but CPython's strsignal reports no text for it.
	sb, err := objects.Call(strsignal, []objects.Object{winSignalAttr(t, "SIGBREAK")})
	if err != nil {
		t.Fatalf("strsignal(SIGBREAK): %v", err)
	}
	if sb != objects.None {
		t.Errorf("strsignal(SIGBREAK) = %v, want None", sb)
	}
	// valid_signals names the Windows signals and excludes ones this host lacks.
	valid := winSignalAttr(t, "valid_signals")
	vs, err := objects.Call(valid, nil)
	if err != nil {
		t.Fatalf("valid_signals(): %v", err)
	}
	has, err := objects.Contains(vs, winSignalAttr(t, "SIGABRT"))
	if err != nil {
		t.Fatalf("SIGABRT in valid_signals: %v", err)
	}
	if b, ok := objects.AsBool(has); !ok || !b {
		t.Errorf("SIGABRT not in valid_signals")
	}
	// 9 is SIGKILL on POSIX, which Windows does not have.
	hasKill, err := objects.Contains(vs, objects.NewInt(9))
	if err != nil {
		t.Fatalf("9 in valid_signals: %v", err)
	}
	if b, ok := objects.AsBool(hasKill); !ok || b {
		t.Errorf("valid_signals should not contain 9 on Windows")
	}
}

func TestSignalWindowsRaiseRunsHandler(t *testing.T) {
	sig := winSignalAttr(t, "signal")
	raise := winSignalAttr(t, "raise_signal")
	sigabrt := winSignalAttr(t, "SIGABRT")
	ran := false
	handler := objects.NewFunc("h", -1, func(args []objects.Object) (objects.Object, error) {
		ran = true
		return objects.None, nil
	})
	if _, err := objects.Call(sig, []objects.Object{sigabrt, handler}); err != nil {
		t.Fatalf("signal(SIGABRT, handler): %v", err)
	}
	defer objects.Call(sig, []objects.Object{sigabrt, winSignalAttr(t, "SIG_DFL")})
	// Windows has no kill(2); raise_signal runs the armed handler directly.
	if _, err := objects.Call(raise, []objects.Object{sigabrt}); err != nil {
		t.Fatalf("raise_signal(SIGABRT): %v", err)
	}
	if !ran {
		t.Errorf("raise_signal(SIGABRT) did not run the installed handler")
	}
}
