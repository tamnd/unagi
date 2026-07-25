//go:build darwin

package runtime

// syslogPlatformConsts holds the syslog facilities darwin defines past the common
// set. darwin's extra facilities (LOG_NETINFO, LOG_REMOTEAUTH, LOG_INSTALL,
// LOG_RA, LOG_LAUNCHD) are Apple-specific and nothing on the floor names them, so
// the table is empty; the common facilities in syslogConsts cover what programs
// use, and a name darwin CPython would not expose either raises a clean
// AttributeError.
var syslogPlatformConsts = []struct {
	name string
	val  int
}{}
