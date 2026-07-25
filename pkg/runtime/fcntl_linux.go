//go:build linux

package runtime

// fcntlPlatformConsts holds the fcntl commands that exist only on linux. The
// pipe-size commands are linux-specific F_LINUX_SPECIFIC_BASE offsets that Go's
// syscall package does not define, so they are the literal request numbers.
// subprocess guards its use of F_SETPIPE_SZ with hasattr, so exposing them here
// (and not on darwin, which has no such commands) matches CPython on both hosts.
var fcntlPlatformConsts = []struct {
	name string
	val  int
}{
	{"F_SETPIPE_SZ", 1031},
	{"F_GETPIPE_SZ", 1032},
}
