//go:build !darwin

package runtime

// The linux (and other non-darwin) getaddrinfo/getnameinfo flag values. The
// corpus host is linux, so these are the values its golden holds against; the
// darwin file carries the dev-host values. AI_MASK and AI_DEFAULT are BSD-only,
// so they are absent here the way they are on linux.
var socketResolveConsts = map[string]int64{
	"AI_PASSIVE":     1,
	"AI_CANONNAME":   2,
	"AI_NUMERICHOST": 4,
	"AI_NUMERICSERV": 1024,
	"AI_ADDRCONFIG":  32,
	"AI_V4MAPPED":    8,
	"AI_ALL":         16,
	"NI_NUMERICHOST": 1,
	"NI_NUMERICSERV": 2,
	"NI_NOFQDN":      4,
	"NI_NAMEREQD":    8,
	"NI_DGRAM":       16,
	"NI_MAXHOST":     1025,
	"NI_MAXSERV":     32,
}
