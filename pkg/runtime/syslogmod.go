//go:build darwin || linux

package runtime

import (
	logsyslog "log/syslog"
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// syslog is the system-logger accelerator. logging.handlers.SysLogHandler is
// network-based and does not need it, but programs and a few tools log directly
// through syslog.openlog/syslog.syslog, and the module is a small, well-defined
// wrapper over the syslog(3) family plus the LOG_* priority, option and facility
// constants. The constant values are the POSIX-standard numbers, identical on
// darwin and linux for the common set exposed here, so the module is a single
// file scoped to those two hosts (Windows CPython has no syslog either); the only
// platform seam is the handful of host-specific facilities in
// syslogPlatformConsts.
//
// Delivery is best-effort. CPython's syslog(3) hands the message to the local
// syslog daemon and writes nothing to stdout or stderr, and a message that the
// daemon cannot be reached for is simply dropped. This mirrors that: the message
// is routed through Go's log/syslog to the local daemon, and any failure to
// connect is swallowed, so a program's observable output is unchanged whether or
// not a daemon is listening, exactly as under CPython.

func init() {
	moduleTable["syslog"] = &moduleEntry{builtin: true, exec: initSyslog}
}

// syslogState is the module-global openlog configuration and priority mask, the
// same process-wide state CPython keeps in the C module.
var syslogState = struct {
	mu       sync.Mutex
	ident    string
	option   int
	facility int
	mask     int
	opened   bool
}{mask: 0xff, facility: logFacilityUser}

const (
	// Priorities (POSIX, host-invariant).
	logEmerg   = 0
	logAlert   = 1
	logCrit    = 2
	logErr     = 3
	logWarning = 4
	logNotice  = 5
	logInfo    = 6
	logDebug   = 7

	// Facility used by default, LOG_USER.
	logFacilityUser = 8
)

// syslogConsts is the name->value table baked to the POSIX-standard numbers.
// Priorities, the option flags and the common facilities carry the same values
// on darwin and linux, so they live in the shared file; syslogPlatformConsts
// adds the host-specific facilities (LOG_AUTHPRIV on linux).
var syslogConsts = []struct {
	name string
	val  int
}{
	{"LOG_EMERG", logEmerg},
	{"LOG_ALERT", logAlert},
	{"LOG_CRIT", logCrit},
	{"LOG_ERR", logErr},
	{"LOG_WARNING", logWarning},
	{"LOG_NOTICE", logNotice},
	{"LOG_INFO", logInfo},
	{"LOG_DEBUG", logDebug},

	{"LOG_PID", 0x01},
	{"LOG_CONS", 0x02},
	{"LOG_ODELAY", 0x04},
	{"LOG_NDELAY", 0x08},
	{"LOG_NOWAIT", 0x10},
	{"LOG_PERROR", 0x20},

	{"LOG_KERN", 0 << 3},
	{"LOG_USER", 1 << 3},
	{"LOG_MAIL", 2 << 3},
	{"LOG_DAEMON", 3 << 3},
	{"LOG_AUTH", 4 << 3},
	{"LOG_SYSLOG", 5 << 3},
	{"LOG_LPR", 6 << 3},
	{"LOG_NEWS", 7 << 3},
	{"LOG_UUCP", 8 << 3},
	{"LOG_CRON", 9 << 3},
	{"LOG_LOCAL0", 16 << 3},
	{"LOG_LOCAL1", 17 << 3},
	{"LOG_LOCAL2", 18 << 3},
	{"LOG_LOCAL3", 19 << 3},
	{"LOG_LOCAL4", 20 << 3},
	{"LOG_LOCAL5", 21 << 3},
	{"LOG_LOCAL6", 22 << 3},
	{"LOG_LOCAL7", 23 << 3},
}

func initSyslog(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("openlog", objects.NewFunc("openlog", -1, syslogOpenlog)); err != nil {
		return err
	}
	if err := set("syslog", objects.NewFunc("syslog", -1, syslogSyslog)); err != nil {
		return err
	}
	if err := set("closelog", objects.NewFunc("closelog", 0, syslogCloselog)); err != nil {
		return err
	}
	if err := set("setlogmask", objects.NewFunc("setlogmask", 1, syslogSetlogmask)); err != nil {
		return err
	}
	if err := set("LOG_MASK", objects.NewFunc("LOG_MASK", 1, syslogLogMask)); err != nil {
		return err
	}
	if err := set("LOG_UPTO", objects.NewFunc("LOG_UPTO", 1, syslogLogUpto)); err != nil {
		return err
	}
	for _, c := range syslogConsts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	for _, c := range syslogPlatformConsts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	return nil
}

// syslogOpenlog is syslog.openlog(ident=sys.argv[0], logoption=0,
// facility=LOG_USER): records the process-wide configuration later syslog calls
// use. All three arguments are keyword-or-positional in CPython, but the floor
// passes them positionally, so this reads them by position.
func syslogOpenlog(args []objects.Object) (objects.Object, error) {
	syslogState.mu.Lock()
	defer syslogState.mu.Unlock()
	if len(args) >= 1 && args[0] != objects.None {
		ident, ok := objects.AsStr(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "openlog() ident must be a string")
		}
		syslogState.ident = ident
	}
	if len(args) >= 2 {
		opt, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "openlog() logoption must be an int")
		}
		syslogState.option = int(opt)
	}
	if len(args) >= 3 {
		fac, ok := objects.AsInt(args[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "openlog() facility must be an int")
		}
		syslogState.facility = int(fac)
	}
	syslogState.opened = true
	return objects.None, nil
}

// syslogSyslog is syslog.syslog(priority=LOG_INFO, message) or
// syslog.syslog(message): a bare message logs at LOG_INFO, a leading int is the
// priority. The priority may carry its own facility in the high bits, otherwise
// the openlog facility applies. Delivery is best-effort and silent, so the call
// never raises for a missing daemon and never writes to stdout or stderr.
func syslogSyslog(args []objects.Object) (objects.Object, error) {
	priority := logInfo
	var msg string
	switch len(args) {
	case 1:
		m, ok := objects.AsStr(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "syslog() message must be a string")
		}
		msg = m
	case 2:
		p, ok := objects.AsInt(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "syslog() priority must be an int")
		}
		priority = int(p)
		m, ok := objects.AsStr(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "syslog() message must be a string")
		}
		msg = m
	default:
		return nil, objects.Raise(objects.TypeError, "syslog() takes 1 or 2 arguments, got %d", len(args))
	}

	syslogState.mu.Lock()
	mask := syslogState.mask
	facility := syslogState.facility
	ident := syslogState.ident
	syslogState.mu.Unlock()

	severity := priority & 0x07
	// The priority mask filters by severity, exactly as CPython's setlogmask.
	if mask&(1<<uint(severity)) == 0 {
		return objects.None, nil
	}
	// A priority that carries its own facility overrides the openlog facility.
	if priority & ^0x07 != 0 {
		facility = priority & ^0x07
	}
	syslogDeliver(facility|severity, ident, msg)
	return objects.None, nil
}

// syslogDeliver hands one message to the local syslog daemon on a fresh
// connection, swallowing any error. syslog is low-frequency, so a per-message
// connection is simpler than caching one and keeps no daemon socket open past
// the call, matching what a program observes either way (nothing on stdout). The
// tag is the openlog ident, matching the C module's behavior of prefixing each
// line with it.
func syslogDeliver(priority int, ident, message string) {
	w, err := logsyslog.New(logsyslog.Priority(priority), ident)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte(message))
	_ = w.Close()
}

// syslogCloselog is syslog.closelog(): resets the module configuration to its
// defaults, as CPython does.
func syslogCloselog(args []objects.Object) (objects.Object, error) {
	syslogState.mu.Lock()
	defer syslogState.mu.Unlock()
	syslogState.ident = ""
	syslogState.option = 0
	syslogState.facility = logFacilityUser
	syslogState.opened = false
	return objects.None, nil
}

// syslogSetlogmask is syslog.setlogmask(maskpri): installs the priority mask and
// returns the previous one. A maskpri of 0 queries without changing, matching
// CPython.
func syslogSetlogmask(args []objects.Object) (objects.Object, error) {
	m, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "setlogmask() argument must be an int")
	}
	syslogState.mu.Lock()
	defer syslogState.mu.Unlock()
	prev := syslogState.mask
	if m != 0 {
		syslogState.mask = int(m)
	}
	return objects.NewInt(int64(prev)), nil
}

// syslogLogMask is syslog.LOG_MASK(pri): the mask for one priority, 1<<pri.
func syslogLogMask(args []objects.Object) (objects.Object, error) {
	pri, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "LOG_MASK() argument must be an int")
	}
	return objects.NewInt(1 << uint(pri)), nil
}

// syslogLogUpto is syslog.LOG_UPTO(pri): the mask for all priorities through pri,
// (1<<(pri+1))-1.
func syslogLogUpto(args []objects.Object) (objects.Object, error) {
	pri, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "LOG_UPTO() argument must be an int")
	}
	return objects.NewInt((1 << uint(pri+1)) - 1), nil
}
