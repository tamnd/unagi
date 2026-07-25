//go:build darwin || linux

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestSyslogModule checks the surface and the mask helpers: the module imports,
// exposes openlog/syslog/closelog/setlogmask/LOG_MASK/LOG_UPTO and the LOG_*
// constants, the priorities carry their POSIX values, LOG_MASK/LOG_UPTO compute
// the standard masks, and openlog/syslog/closelog run without raising (delivery
// is best-effort and observable only in the system log).
func TestSyslogModule(t *testing.T) {
	mo, err := ImportModule("syslog")
	if err != nil {
		t.Fatalf("import syslog: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("syslog.%s: %v", name, err)
		}
		return v
	}
	constInt := func(name string) int64 {
		t.Helper()
		n, ok := objects.AsInt(attr(name))
		if !ok {
			t.Fatalf("syslog.%s is not an int", name)
		}
		return n
	}
	// Priorities are POSIX-fixed.
	for name, want := range map[string]int64{
		"LOG_EMERG": 0, "LOG_ALERT": 1, "LOG_CRIT": 2, "LOG_ERR": 3,
		"LOG_WARNING": 4, "LOG_NOTICE": 5, "LOG_INFO": 6, "LOG_DEBUG": 7,
		"LOG_USER": 8, "LOG_LOCAL0": 128,
	} {
		if got := constInt(name); got != want {
			t.Fatalf("syslog.%s = %d, want %d", name, got, want)
		}
	}

	call := func(name string, args ...objects.Object) objects.Object {
		t.Helper()
		v, err := objects.Call(attr(name), args)
		if err != nil {
			t.Fatalf("syslog.%s: %v", name, err)
		}
		return v
	}
	// LOG_MASK(INFO) = 1<<6, LOG_UPTO(INFO) = (1<<7)-1.
	if got, _ := objects.AsInt(call("LOG_MASK", objects.NewInt(constInt("LOG_INFO")))); got != 64 {
		t.Fatalf("LOG_MASK(LOG_INFO) = %d, want 64", got)
	}
	if got, _ := objects.AsInt(call("LOG_UPTO", objects.NewInt(constInt("LOG_INFO")))); got != 127 {
		t.Fatalf("LOG_UPTO(LOG_INFO) = %d, want 127", got)
	}

	// openlog/syslog/closelog run without raising; setlogmask returns the prior
	// mask and a query with 0 does not change it.
	call("openlog", objects.NewStr("unagitest"), objects.NewInt(constInt("LOG_PID")), objects.NewInt(constInt("LOG_USER")))
	call("syslog", objects.NewInt(constInt("LOG_INFO")), objects.NewStr("test message"))
	call("syslog", objects.NewStr("bare message"))
	prev, _ := objects.AsInt(call("setlogmask", objects.NewInt(0)))
	if prev < 0 {
		t.Fatalf("setlogmask(0) returned negative mask %d", prev)
	}
	call("closelog")
}
