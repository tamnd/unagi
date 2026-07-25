//go:build darwin

package runtime

// The getaddrinfo/getnameinfo flag constants carry different numeric values on
// darwin than on linux, the same platform divergence slice 1 handled for the
// address families. These are the darwin values; the non-darwin file carries the
// linux ones. A program built on either host sees the value its C library uses.
var socketResolveConsts = map[string]int64{
	"AI_PASSIVE":     1,
	"AI_CANONNAME":   2,
	"AI_NUMERICHOST": 4,
	"AI_NUMERICSERV": 4096,
	"AI_ADDRCONFIG":  1024,
	"AI_V4MAPPED":    2048,
	"AI_ALL":         256,
	"AI_MASK":        5127,
	"AI_DEFAULT":     1536,
	"NI_NUMERICHOST": 2,
	"NI_NUMERICSERV": 8,
	"NI_NOFQDN":      1,
	"NI_NAMEREQD":    4,
	"NI_DGRAM":       16,
	"NI_MAXHOST":     1025,
	"NI_MAXSERV":     32,
}
