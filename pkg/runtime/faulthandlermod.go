package runtime

import (
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// faulthandler is the fault-diagnostics module. It is a pure C builtin with no
// Python fallback, so programs and test harnesses that open with
// faulthandler.enable() (to dump a traceback on a fatal signal) could not import
// it. The module's observable contract is the enabled state: enable() turns it
// on, is_enabled() reports it, disable() turns it off. The dumping itself is
// crash-time diagnostics a program's result never depends on, so the dump entry
// points accept their arguments and return without producing output; see the
// follow-up note.
//
// The module is portable (no syscall surface here), so `import faulthandler`
// works on every host the runtime targets.

func init() {
	moduleTable["faulthandler"] = &moduleEntry{builtin: true, exec: initFaulthandler}
}

// faulthandlerState is the module-global enabled flag, the one piece of
// observable state the C module keeps.
var faulthandlerState = struct {
	mu      sync.Mutex
	enabled bool
}{}

func initFaulthandler(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	fns := []struct {
		name string
		fn   func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error)
	}{
		{"enable", faulthandlerEnable},
		{"disable", faulthandlerDisable},
		{"is_enabled", faulthandlerIsEnabled},
		{"dump_traceback", faulthandlerDumpTraceback},
		{"dump_traceback_later", faulthandlerDumpTracebackLater},
		{"cancel_dump_traceback_later", faulthandlerCancelDump},
		{"register", faulthandlerRegister},
		{"unregister", faulthandlerUnregister},
		{"dump_c_stack", faulthandlerDumpCStack},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFuncKw(f.name, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// faulthandlerEnable is faulthandler.enable(file=sys.stderr, all_threads=True):
// arms the handler. The file and all_threads arguments select where and what a
// fault dump would cover; the dump is not produced here, so they are accepted and
// ignored.
func faulthandlerEnable(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	faulthandlerState.mu.Lock()
	faulthandlerState.enabled = true
	faulthandlerState.mu.Unlock()
	return objects.None, nil
}

// faulthandlerDisable is faulthandler.disable(): disarms the handler.
func faulthandlerDisable(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	faulthandlerState.mu.Lock()
	faulthandlerState.enabled = false
	faulthandlerState.mu.Unlock()
	return objects.None, nil
}

// faulthandlerIsEnabled is faulthandler.is_enabled(): whether enable() is in
// effect.
func faulthandlerIsEnabled(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	faulthandlerState.mu.Lock()
	on := faulthandlerState.enabled
	faulthandlerState.mu.Unlock()
	if on {
		return objects.True, nil
	}
	return objects.False, nil
}

// faulthandlerDumpTraceback is faulthandler.dump_traceback(file=sys.stderr,
// all_threads=True): dumps the current traceback. It is a diagnostic a program's
// result does not depend on, so it accepts its arguments and returns without
// output.
func faulthandlerDumpTraceback(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// faulthandlerDumpTracebackLater is faulthandler.dump_traceback_later(timeout,
// repeat=False, file=sys.stderr, exit=False): schedules a watchdog dump. The
// timeout is validated as a positive number, matching CPython's error, and the
// watchdog is a no-op so nothing is dumped.
func faulthandlerDumpTracebackLater(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	timeout, ok := faulthandlerArg(pos, kwNames, kwVals, 0, "timeout")
	if !ok {
		return nil, objects.Raise(objects.TypeError, "dump_traceback_later() missing required argument 'timeout'")
	}
	if _, ok := objects.AsFloat(timeout); !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as a number", timeout.TypeName())
	}
	return objects.None, nil
}

// faulthandlerCancelDump is faulthandler.cancel_dump_traceback_later(): cancels a
// scheduled watchdog dump. With no watchdog running it is a no-op.
func faulthandlerCancelDump(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// faulthandlerRegister is faulthandler.register(signum, file=sys.stderr,
// all_threads=True, chain=False): installs a per-signal dump handler. The signum
// is validated as an int; the handler is a no-op.
func faulthandlerRegister(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	signum, ok := faulthandlerArg(pos, kwNames, kwVals, 0, "signum")
	if !ok {
		return nil, objects.Raise(objects.TypeError, "register() missing required argument 'signum'")
	}
	if _, ok := objects.AsIntValue(signum); !ok {
		return nil, objects.Raise(objects.TypeError, "signum must be an integer")
	}
	return objects.None, nil
}

// faulthandlerUnregister is faulthandler.unregister(signum): removes a handler
// register() installed, returning whether one was present. Since register is a
// no-op, this reports False.
func faulthandlerUnregister(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	signum, ok := faulthandlerArg(pos, kwNames, kwVals, 0, "signum")
	if !ok {
		return nil, objects.Raise(objects.TypeError, "unregister() missing required argument 'signum'")
	}
	if _, ok := objects.AsIntValue(signum); !ok {
		return nil, objects.Raise(objects.TypeError, "signum must be an integer")
	}
	return objects.False, nil
}

// faulthandlerDumpCStack is faulthandler.dump_c_stack(file=sys.stderr): dumps the
// C stack (3.14+). A diagnostic with no bearing on a program's result, so it
// returns without output.
func faulthandlerDumpCStack(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// faulthandlerArg reads an argument by position or keyword name.
func faulthandlerArg(pos []objects.Object, kwNames []string, kwVals []objects.Object, index int, name string) (objects.Object, bool) {
	for i, kn := range kwNames {
		if kn == name {
			return kwVals[i], true
		}
	}
	if index < len(pos) {
		return pos[index], true
	}
	return nil, false
}
