//go:build !darwin && !linux

package runtime

import "syscall"

// signalNames is the fallback signal set for hosts that are neither darwin nor
// linux. It carries only the two signals every platform Go targets defines,
// SIGINT and SIGTERM, which is enough for the module to import and for the
// programs that only name those two. A host that needs more gets its own table
// the way darwin and linux do.
var signalNames = []signalDef{
	{"SIGINT", int(syscall.SIGINT), "Interrupt"},
	{"SIGTERM", int(syscall.SIGTERM), "Terminated"},
}

// nsigValue is a conservative NSIG for the fallback host.
const nsigValue = 32

// strsignalIncludesNumber leaves the description bare on the fallback host.
const strsignalIncludesNumber = false
