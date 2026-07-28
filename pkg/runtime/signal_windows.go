//go:build windows

package runtime

import "github.com/tamnd/unagi/pkg/objects"

// signalExtraConsts are the console-control codes CPython's _signal exposes on
// Windows. signal.py folds any CTRL_* name into its Signals enum, so they must
// be present as module ints; they are not deliverable signals, so they are kept
// out of signalNames and never pass knownSignal.
var signalExtraConsts = []signalConst{
	{"CTRL_C_EVENT", 0},
	{"CTRL_BREAK_EVENT", 1},
}

// raiseSignalOS is signal.raise_signal on Windows. CPython raises through the C
// runtime's raise(), which delivers to this process; Go has no kill(2) on
// Windows and cannot re-enter the C handler table without cgo, so this runs the
// armed Python handler directly instead. That reproduces the observable effect
// of raising a signal you have handled (the callable runs); SIG_IGN and SIG_DFL
// are a no-op, since there is no OS default to trigger here. Delivery of real
// OS signals still flows through os/signal.Notify for the signals Go surfaces on
// Windows (Ctrl-C as SIGINT, SIGBREAK, SIGTERM); this path only covers the
// self-raise case.
func raiseSignalOS(sn int) (objects.Object, error) {
	signalReg.mu.Lock()
	h := signalReg.handlers[sn]
	th := dispatchThreadLocked()
	signalReg.mu.Unlock()
	if h == nil {
		return objects.None, nil
	}
	if _, isInt := objects.AsInt(h); isInt {
		// SIG_DFL or SIG_IGN: nothing to run.
		return objects.None, nil
	}
	if _, err := objects.CallT(th, h, []objects.Object{objects.NewInt(int64(sn)), objects.None}); err != nil {
		return nil, err
	}
	return objects.None, nil
}
