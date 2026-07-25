//go:build darwin

package runtime

import "syscall"

// signalNames is the darwin signal set, name to number to the description
// darwin's strsignal returns. The numbers come from Go's syscall constants so
// they track the build host's headers, the same sourcing errno and the socket
// constants use. The descriptions are the exact text CPython hands back from
// signal.strsignal on darwin, which pairs the phrase with the number, so
// strsignalIncludesNumber is true here and the number is appended by the
// portable formatter.
var signalNames = []signalDef{
	{"SIGHUP", int(syscall.SIGHUP), "Hangup"},
	{"SIGINT", int(syscall.SIGINT), "Interrupt"},
	{"SIGQUIT", int(syscall.SIGQUIT), "Quit"},
	{"SIGILL", int(syscall.SIGILL), "Illegal instruction"},
	{"SIGTRAP", int(syscall.SIGTRAP), "Trace/BPT trap"},
	{"SIGABRT", int(syscall.SIGABRT), "Abort trap"},
	{"SIGEMT", int(syscall.SIGEMT), "EMT trap"},
	{"SIGFPE", int(syscall.SIGFPE), "Floating point exception"},
	{"SIGKILL", int(syscall.SIGKILL), "Killed"},
	{"SIGBUS", int(syscall.SIGBUS), "Bus error"},
	{"SIGSEGV", int(syscall.SIGSEGV), "Segmentation fault"},
	{"SIGSYS", int(syscall.SIGSYS), "Bad system call"},
	{"SIGPIPE", int(syscall.SIGPIPE), "Broken pipe"},
	{"SIGALRM", int(syscall.SIGALRM), "Alarm clock"},
	{"SIGTERM", int(syscall.SIGTERM), "Terminated"},
	{"SIGURG", int(syscall.SIGURG), "Urgent I/O condition"},
	{"SIGSTOP", int(syscall.SIGSTOP), "Suspended (signal)"},
	{"SIGTSTP", int(syscall.SIGTSTP), "Suspended"},
	{"SIGCONT", int(syscall.SIGCONT), "Continued"},
	{"SIGCHLD", int(syscall.SIGCHLD), "Child exited"},
	{"SIGTTIN", int(syscall.SIGTTIN), "Stopped (tty input)"},
	{"SIGTTOU", int(syscall.SIGTTOU), "Stopped (tty output)"},
	{"SIGIO", int(syscall.SIGIO), "I/O possible"},
	{"SIGXCPU", int(syscall.SIGXCPU), "Cputime limit exceeded"},
	{"SIGXFSZ", int(syscall.SIGXFSZ), "Filesize limit exceeded"},
	{"SIGVTALRM", int(syscall.SIGVTALRM), "Virtual timer expired"},
	{"SIGPROF", int(syscall.SIGPROF), "Profiling timer expired"},
	{"SIGWINCH", int(syscall.SIGWINCH), "Window size changes"},
	{"SIGINFO", int(syscall.SIGINFO), "Information request"},
	{"SIGUSR1", int(syscall.SIGUSR1), "User defined signal 1"},
	{"SIGUSR2", int(syscall.SIGUSR2), "User defined signal 2"},
}

// nsigValue is darwin's NSIG, one past the highest signal number.
const nsigValue = 32

// strsignalIncludesNumber records that darwin's strsignal appends ": <num>" to
// the description, which linux's does not.
const strsignalIncludesNumber = true
