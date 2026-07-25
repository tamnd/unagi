//go:build linux

package runtime

// syslogPlatformConsts holds the syslog facilities linux defines past the common
// set. LOG_AUTHPRIV (10<<3) is the private authorization facility glibc adds;
// darwin has no such facility, so exposing it only here matches CPython on both.
var syslogPlatformConsts = []struct {
	name string
	val  int
}{
	{"LOG_AUTHPRIV", 10 << 3},
}
