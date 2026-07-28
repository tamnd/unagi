//go:build !windows

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// signalExtraConsts is empty off Windows: unix exposes no non-signal control
// codes through _signal.
var signalExtraConsts []signalConst

// raiseSignalOS is signal.raise_signal on unix: send the signal to the current
// process through kill(2), so an armed handler runs on its delivery goroutine
// exactly as it would for a signal from outside the process.
func raiseSignalOS(sn int) (objects.Object, error) {
	if err := syscall.Kill(syscall.Getpid(), syscall.Signal(sn)); err != nil {
		return nil, objects.Raise("OSError", "[Errno %d] %s", int(err.(syscall.Errno)), err.Error())
	}
	return objects.None, nil
}
