//go:build !windows

package runtime

import (
	"fmt"
	"os"
	osignal "os/signal"
	"sync"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// _signal is the accelerator behind the signal module. signal.py imports it,
// star-imports its names, turns the SIG* integers into the Signals IntEnum and
// wraps signal()/getsignal() so callers see enum members; everything the wrap
// needs, the constants and the two functions, lives here. The module exposes
// the POSIX signal numbers per host (from Go's syscall constants), SIG_DFL and
// SIG_IGN, NSIG, and signal/getsignal/raise_signal/strsignal/valid_signals/
// default_int_handler.
//
// Delivery runs the way the rest of unagi's concurrency does. There is no GIL
// and no bytecode loop to check a flag between instructions, so a handler set
// for a signal is armed through os/signal.Notify and run on a dispatch
// goroutine when the signal arrives, the same shape threading uses to run a
// target off the main goroutine. This differs from CPython, where the handler
// runs on the main thread between bytecodes; it is the honest model for
// compiled code. SIG_IGN and SIG_DFL go straight to the OS through
// signal.Ignore and signal.Reset and never call back into Python.

// sigDfl and sigIgn are the two sentinel handler values, the integers CPython
// exposes as SIG_DFL and SIG_IGN.
const (
	sigDfl = 0
	sigIgn = 1
)

// signalDef is one row of a per-host signal table: the Python name, the number
// Go's syscall gives it on this host, and the description strsignal returns.
type signalDef struct {
	name string
	num  int
	desc string
}

func init() {
	moduleTable["_signal"] = &moduleEntry{builtin: true, exec: initSignal}
}

// signalReg is the live handler registry. handlers maps a signal number to the
// Python callable installed for it; a number absent from the map, or holding a
// sentinel, has no Python handler armed. chans holds the os/signal channel
// feeding each armed goroutine so the handler can be torn down when it is
// replaced by SIG_DFL or SIG_IGN. dispatch is the one thread state every
// handler call runs under, created on first use.
var signalReg = struct {
	mu       sync.Mutex
	handlers map[int]objects.Object
	chans    map[int]chan os.Signal
	dispatch *objects.Thread
}{
	handlers: map[int]objects.Object{},
	chans:    map[int]chan os.Signal{},
}

// defaultIntHandler is the module's default_int_handler, the callable CPython
// installs for SIGINT at startup. getsignal(SIGINT) hands it back, so it is
// seeded as SIGINT's handler value without arming OS delivery: the program's
// own Ctrl-C handling stays in charge until user code calls signal().
var defaultIntHandler = objects.NewFunc("default_int_handler", -1, func(args []objects.Object) (objects.Object, error) {
	return nil, objects.Raise("KeyboardInterrupt", "")
})

func initSignal(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	for _, s := range signalNames {
		if err := set(s.name, objects.NewInt(int64(s.num))); err != nil {
			return err
		}
	}
	if err := set("SIG_DFL", objects.NewInt(sigDfl)); err != nil {
		return err
	}
	if err := set("SIG_IGN", objects.NewInt(sigIgn)); err != nil {
		return err
	}
	if err := set("NSIG", objects.NewInt(nsigValue)); err != nil {
		return err
	}
	if err := set("default_int_handler", defaultIntHandler); err != nil {
		return err
	}
	// Seed SIGINT's handler the way CPython does, so getsignal(SIGINT) reports
	// default_int_handler rather than SIG_DFL. Delivery is not armed here.
	if sig, ok := signalNumber("SIGINT"); ok {
		signalReg.mu.Lock()
		signalReg.handlers[sig] = defaultIntHandler
		signalReg.mu.Unlock()
	}
	funcs := []struct {
		name string
		fn   objects.Object
	}{
		{"signal", objects.NewFuncT("signal", -1, signalSignal)},
		{"getsignal", objects.NewFunc("getsignal", 1, signalGetsignal)},
		{"raise_signal", objects.NewFunc("raise_signal", 1, signalRaise)},
		{"strsignal", objects.NewFunc("strsignal", 1, signalStrsignal)},
		{"valid_signals", objects.NewFunc("valid_signals", 0, signalValid)},
	}
	for _, f := range funcs {
		if err := set(f.name, f.fn); err != nil {
			return err
		}
	}
	return nil
}

// signalNumber returns the number for a signal name on this host.
func signalNumber(name string) (int, bool) {
	for _, s := range signalNames {
		if s.name == name {
			return s.num, true
		}
	}
	return 0, false
}

// knownSignal reports whether n is a signal number this host names, the bound
// signal() and raise_signal validate against.
func knownSignal(n int) bool {
	for _, s := range signalNames {
		if s.num == n {
			return true
		}
	}
	return false
}

// uncatchable reports whether a signal cannot have a handler installed: SIGKILL
// and SIGSTOP are enforced by the kernel, and CPython raises OSError EINVAL for
// them, which this matches.
func uncatchable(n int) bool {
	if s, ok := signalNumber("SIGKILL"); ok && s == n {
		return true
	}
	if s, ok := signalNumber("SIGSTOP"); ok && s == n {
		return true
	}
	return false
}

// signalSignal is signal.signal at the _signal level: install a handler for a
// signal and return the one it replaces. It runs only on the main thread, takes
// SIG_IGN, SIG_DFL or a callable, and rejects the uncatchable signals the way
// CPython does. A callable arms os/signal delivery; the two sentinels hand the
// signal to the OS and tear any armed goroutine down.
func signalSignal(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "signal() takes exactly 2 arguments (%d given)", len(args))
	}
	if t != nil && !t.IsMain() {
		return nil, objects.Raise(objects.ValueError, "signal only works in main thread of the main interpreter")
	}
	sig, ok := objects.AsIntValue(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "signal handler:the signalnum argument must be an int")
	}
	sn := int(sig)
	if !knownSignal(sn) {
		return nil, objects.Raise(objects.ValueError, "signal number out of range")
	}
	handler := args[1]
	hv, isInt := objects.AsInt(handler)
	if !isInt && !objects.Callable(handler) {
		return nil, objects.Raise(objects.TypeError, "signal handler must be signal.SIG_IGN, signal.SIG_DFL, or a callable object")
	}
	if isInt && hv != sigDfl && hv != sigIgn {
		return nil, objects.Raise(objects.TypeError, "signal handler must be signal.SIG_IGN, signal.SIG_DFL, or a callable object")
	}
	if uncatchable(sn) {
		return nil, objects.Raise("OSError", "[Errno 22] Invalid argument")
	}

	signalReg.mu.Lock()
	defer signalReg.mu.Unlock()
	prev := currentHandlerLocked(sn)
	sysSig := syscall.Signal(sn)
	switch {
	case isInt && hv == sigIgn:
		disarmLocked(sn)
		osignal.Ignore(sysSig)
		signalReg.handlers[sn] = objects.NewInt(sigIgn)
	case isInt && hv == sigDfl:
		disarmLocked(sn)
		osignal.Reset(sysSig)
		signalReg.handlers[sn] = objects.NewInt(sigDfl)
	default:
		armLocked(sn, sysSig)
		signalReg.handlers[sn] = handler
	}
	return prev, nil
}

// currentHandlerLocked returns the handler value getsignal reports for a
// signal: the stored callable or sentinel, or SIG_DFL when nothing is
// installed. The registry lock is held.
func currentHandlerLocked(sn int) objects.Object {
	if h, ok := signalReg.handlers[sn]; ok {
		return h
	}
	return objects.NewInt(sigDfl)
}

// armLocked makes sure a delivery goroutine is running for a signal, so a
// callable handler installed for it is invoked when it arrives. A second call
// for an already-armed signal is a no-op: the running goroutine reads the
// current handler off the registry each time, so replacing one callable with
// another needs no new goroutine. The registry lock is held.
func armLocked(sn int, sysSig syscall.Signal) {
	if _, ok := signalReg.chans[sn]; ok {
		return
	}
	ch := make(chan os.Signal, 1)
	signalReg.chans[sn] = ch
	osignal.Notify(ch, sysSig)
	go func() {
		for range ch {
			signalReg.mu.Lock()
			h := signalReg.handlers[sn]
			th := dispatchThreadLocked()
			signalReg.mu.Unlock()
			if h == nil {
				continue
			}
			if _, isInt := objects.AsInt(h); isInt {
				// A sentinel slipped in between a signal arriving and the
				// handler being torn down; the OS state already reflects it.
				continue
			}
			if _, err := objects.CallT(th, h, []objects.Object{objects.NewInt(int64(sn)), objects.None}); err != nil {
				if objects.ThreadExcHook != nil {
					objects.ThreadExcHook(err)
				}
			}
		}
	}()
}

// disarmLocked stops any delivery goroutine for a signal, used when a callable
// handler is replaced by SIG_DFL or SIG_IGN. Closing the channel ends the
// goroutine's range loop. The registry lock is held.
func disarmLocked(sn int) {
	ch, ok := signalReg.chans[sn]
	if !ok {
		return
	}
	osignal.Stop(ch)
	close(ch)
	delete(signalReg.chans, sn)
}

// dispatchThreadLocked returns the thread state signal handlers run under,
// creating it on first use. One state is enough: handlers run one at a time as
// their channel delivers, so they never share it concurrently. The registry
// lock is held.
func dispatchThreadLocked() *objects.Thread {
	if signalReg.dispatch == nil {
		signalReg.dispatch = objects.NewThread("SignalDispatch", true)
	}
	return signalReg.dispatch
}

// signalGetsignal is signal.getsignal at the _signal level: the handler value
// installed for a signal, or SIG_DFL when none is. It does not validate the
// number beyond the range check, matching CPython, which returns the stored
// slot for any in-range signal.
func signalGetsignal(args []objects.Object) (objects.Object, error) {
	sig, ok := objects.AsIntValue(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	sn := int(sig)
	if !knownSignal(sn) {
		return nil, objects.Raise(objects.ValueError, "signal number out of range")
	}
	signalReg.mu.Lock()
	defer signalReg.mu.Unlock()
	return currentHandlerLocked(sn), nil
}

// signalRaise is signal.raise_signal: send a signal to the current process. It
// goes through the OS, so an armed handler runs on its delivery goroutine just
// as it would for a signal from outside the process.
func signalRaise(args []objects.Object) (objects.Object, error) {
	sig, ok := objects.AsIntValue(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	sn := int(sig)
	if !knownSignal(sn) {
		return nil, objects.Raise(objects.ValueError, "signal number out of range")
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.Signal(sn)); err != nil {
		return nil, objects.Raise("OSError", "[Errno %d] %s", int(err.(syscall.Errno)), err.Error())
	}
	return objects.None, nil
}

// signalStrsignal is signal.strsignal: the human description of a signal, or
// None for a number this host does not name, matching CPython's None-for-
// unknown. Darwin pairs the description with the number and linux does not, a
// split the per-host table records.
func signalStrsignal(args []objects.Object) (objects.Object, error) {
	sig, ok := objects.AsIntValue(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	sn := int(sig)
	for _, s := range signalNames {
		if s.num == sn {
			text := s.desc
			if strsignalIncludesNumber {
				text = fmt.Sprintf("%s: %d", s.desc, sn)
			}
			return objects.NewStr(text), nil
		}
	}
	return objects.None, nil
}

// signalValid is signal.valid_signals: the set of signal numbers valid on this
// host. signal.py maps each to a Signals member, so returning the plain ints
// this table names is enough.
func signalValid(args []objects.Object) (objects.Object, error) {
	nums := make([]objects.Object, 0, len(signalNames))
	for _, s := range signalNames {
		nums = append(nums, objects.NewInt(int64(s.num)))
	}
	return objects.NewSet(nums)
}
