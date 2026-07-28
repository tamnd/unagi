//go:build windows

package runtime

// signalNames is the Windows signal set CPython exposes, pinned to the MS C
// runtime numbers it reports rather than Go's syscall constants, which differ
// (Go's syscall.SIGABRT is 6, the CRT's is 22). The descriptions are the exact
// text CPython's strsignal returns on Windows; SIGBREAK has none, so its empty
// description makes strsignal return None. Windows strsignal does not append the
// number, so strsignalIncludesNumber is false.
//
// This is the full set signal.valid_signals() reports on Windows. SIGKILL and
// SIGSTOP are absent, so nothing is uncatchable here.
var signalNames = []signalDef{
	{"SIGINT", 2, "Interrupt"},
	{"SIGILL", 4, "Illegal instruction"},
	{"SIGFPE", 8, "Floating-point exception"},
	{"SIGSEGV", 11, "Segmentation fault"},
	{"SIGTERM", 15, "Terminated"},
	{"SIGBREAK", 21, ""},
	{"SIGABRT", 22, "Aborted"},
}

// nsigValue is Windows' NSIG, one past the highest signal number CPython reports.
const nsigValue = 23

// strsignalIncludesNumber records that Windows strsignal returns the bare
// description with no ": <num>" suffix.
const strsignalIncludesNumber = false
