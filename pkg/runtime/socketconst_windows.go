//go:build windows

package runtime

// The Windows _socket constants, pinned to the values CPython 3.14.6 exposes on
// windows/amd64 rather than derived from Go's syscall, which does not carry all
// of them (SO_ERROR, SO_TYPE, SOCK_RDM, IPPROTO_ICMP/RAW and the MSG_* flags are
// absent there). Two differences from unix are load-bearing: Windows has no
// AF_UNIX in its socket module, so it is omitted here; and the SO_* option codes
// are the winsock numbers (SO_ERROR 0x1007, SO_TYPE 0x1008, SO_RCVBUF 0x1002,
// SO_SNDBUF 0x1001, SO_LINGER 0x80) rather than the small linux ones.
// servicesFilePath is where getservbyport reads the port-to-name table on
// Windows, the file the C library consults there.
const servicesFilePath = `C:\Windows\System32\drivers\etc\services`

var socketModuleConsts = []socketConst{
	{"AF_UNSPEC", 0},
	{"AF_INET", 2},
	{"AF_INET6", 23},
	{"SOCK_STREAM", 1},
	{"SOCK_DGRAM", 2},
	{"SOCK_RAW", 3},
	{"SOCK_RDM", 4},
	{"SOCK_SEQPACKET", 5},
	{"SOL_SOCKET", 65535},
	{"IPPROTO_IP", 0},
	{"IPPROTO_ICMP", 1},
	{"IPPROTO_TCP", 6},
	{"IPPROTO_UDP", 17},
	{"IPPROTO_IPV6", 41},
	{"IPPROTO_RAW", 255},
	{"SO_REUSEADDR", 4},
	{"SO_KEEPALIVE", 8},
	{"SO_BROADCAST", 32},
	{"SO_ERROR", 4103},
	{"SO_TYPE", 4104},
	{"SO_LINGER", 128},
	{"SO_RCVBUF", 4098},
	{"SO_SNDBUF", 4097},
	{"TCP_NODELAY", 1},
	{"SHUT_RD", 0},
	{"SHUT_WR", 1},
	{"SHUT_RDWR", 2},
	{"MSG_PEEK", 2},
	{"MSG_WAITALL", 8},
	{"MSG_DONTROUTE", 4},
	{"MSG_OOB", 1},
}

// socketResolveConsts is the Windows getaddrinfo/getnameinfo flag table, the
// winsock values CPython reports there. AI_MASK and AI_DEFAULT are BSD-only, so
// Windows omits them the way linux does; AI_NUMERICSERV is 8 here, not the linux
// 1024 or darwin 4096.
var socketResolveConsts = map[string]int64{
	"AI_PASSIVE":     1,
	"AI_CANONNAME":   2,
	"AI_NUMERICHOST": 4,
	"AI_NUMERICSERV": 8,
	"AI_ADDRCONFIG":  1024,
	"AI_V4MAPPED":    2048,
	"AI_ALL":         256,
	"NI_NUMERICHOST": 2,
	"NI_NUMERICSERV": 8,
	"NI_NOFQDN":      1,
	"NI_NAMEREQD":    4,
	"NI_DGRAM":       16,
	"NI_MAXHOST":     1025,
	"NI_MAXSERV":     32,
}
