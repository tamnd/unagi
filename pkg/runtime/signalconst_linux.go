//go:build linux

package runtime

import "syscall"

// signalNames is the linux signal set. The numbers come from Go's syscall
// constants for the build host, and the descriptions are the text glibc's
// strsignal returns, which unlike darwin's carries no trailing number, so
// strsignalIncludesNumber is false. Linux has SIGSTKFLT and SIGPWR where
// darwin has SIGEMT and SIGINFO; the two tables are otherwise the common POSIX
// set.
var signalNames = []signalDef{
	{"SIGHUP", int(syscall.SIGHUP), "Hangup"},
	{"SIGINT", int(syscall.SIGINT), "Interrupt"},
	{"SIGQUIT", int(syscall.SIGQUIT), "Quit"},
	{"SIGILL", int(syscall.SIGILL), "Illegal instruction"},
	{"SIGTRAP", int(syscall.SIGTRAP), "Trace/breakpoint trap"},
	{"SIGABRT", int(syscall.SIGABRT), "Aborted"},
	{"SIGBUS", int(syscall.SIGBUS), "Bus error"},
	{"SIGFPE", int(syscall.SIGFPE), "Floating point exception"},
	{"SIGKILL", int(syscall.SIGKILL), "Killed"},
	{"SIGUSR1", int(syscall.SIGUSR1), "User defined signal 1"},
	{"SIGSEGV", int(syscall.SIGSEGV), "Segmentation fault"},
	{"SIGUSR2", int(syscall.SIGUSR2), "User defined signal 2"},
	{"SIGPIPE", int(syscall.SIGPIPE), "Broken pipe"},
	{"SIGALRM", int(syscall.SIGALRM), "Alarm clock"},
	{"SIGTERM", int(syscall.SIGTERM), "Terminated"},
	{"SIGSTKFLT", int(syscall.SIGSTKFLT), "Stack fault"},
	{"SIGCHLD", int(syscall.SIGCHLD), "Child exited"},
	{"SIGCONT", int(syscall.SIGCONT), "Continued"},
	{"SIGSTOP", int(syscall.SIGSTOP), "Stopped (signal)"},
	{"SIGTSTP", int(syscall.SIGTSTP), "Stopped"},
	{"SIGTTIN", int(syscall.SIGTTIN), "Stopped (tty input)"},
	{"SIGTTOU", int(syscall.SIGTTOU), "Stopped (tty output)"},
	{"SIGURG", int(syscall.SIGURG), "Urgent I/O condition"},
	{"SIGXCPU", int(syscall.SIGXCPU), "CPU time limit exceeded"},
	{"SIGXFSZ", int(syscall.SIGXFSZ), "File size limit exceeded"},
	{"SIGVTALRM", int(syscall.SIGVTALRM), "Virtual timer expired"},
	{"SIGPROF", int(syscall.SIGPROF), "Profiling timer expired"},
	{"SIGWINCH", int(syscall.SIGWINCH), "Window changed"},
	{"SIGIO", int(syscall.SIGIO), "I/O possible"},
	{"SIGPWR", int(syscall.SIGPWR), "Power failure"},
	{"SIGSYS", int(syscall.SIGSYS), "Bad system call"},
}

// nsigValue is linux's NSIG, one past the highest standard signal number.
const nsigValue = 65

// strsignalIncludesNumber records that linux's strsignal returns the bare
// description with no trailing number.
const strsignalIncludesNumber = false
