//go:build !windows

package runtime

import "syscall"

// servicesFilePath is where getservbyport reads the port-to-name table, the
// path getservbyport itself consults on unix.
const servicesFilePath = "/etc/services"

// socketModuleConsts is the unix _socket constant table. Every value comes from
// syscall, so a darwin build sees darwin's numbers and a linux build sees
// linux's, the way CPython's socket module reflects the C library it was built
// against. Windows differs enough (no AF_UNIX, winsock SO_* codes) that it has
// its own literal table in socketmodconst_windows.go.
var socketModuleConsts = []socketConst{
	{"AF_UNSPEC", syscall.AF_UNSPEC},
	{"AF_INET", syscall.AF_INET},
	{"AF_INET6", syscall.AF_INET6},
	{"AF_UNIX", syscall.AF_UNIX},
	{"SOCK_STREAM", syscall.SOCK_STREAM},
	{"SOCK_DGRAM", syscall.SOCK_DGRAM},
	{"SOCK_RAW", syscall.SOCK_RAW},
	{"SOCK_RDM", syscall.SOCK_RDM},
	{"SOCK_SEQPACKET", syscall.SOCK_SEQPACKET},
	{"SOL_SOCKET", syscall.SOL_SOCKET},
	{"IPPROTO_IP", syscall.IPPROTO_IP},
	{"IPPROTO_ICMP", syscall.IPPROTO_ICMP},
	{"IPPROTO_TCP", syscall.IPPROTO_TCP},
	{"IPPROTO_UDP", syscall.IPPROTO_UDP},
	{"IPPROTO_IPV6", syscall.IPPROTO_IPV6},
	{"IPPROTO_RAW", syscall.IPPROTO_RAW},
	{"SO_REUSEADDR", syscall.SO_REUSEADDR},
	{"SO_KEEPALIVE", syscall.SO_KEEPALIVE},
	{"SO_BROADCAST", syscall.SO_BROADCAST},
	{"SO_ERROR", syscall.SO_ERROR},
	{"SO_TYPE", syscall.SO_TYPE},
	{"SO_LINGER", syscall.SO_LINGER},
	{"SO_RCVBUF", syscall.SO_RCVBUF},
	{"SO_SNDBUF", syscall.SO_SNDBUF},
	{"TCP_NODELAY", syscall.TCP_NODELAY},
	{"SHUT_RD", syscall.SHUT_RD},
	{"SHUT_WR", syscall.SHUT_WR},
	{"SHUT_RDWR", syscall.SHUT_RDWR},
	{"MSG_PEEK", syscall.MSG_PEEK},
	{"MSG_WAITALL", syscall.MSG_WAITALL},
	{"MSG_DONTROUTE", syscall.MSG_DONTROUTE},
	{"MSG_OOB", syscall.MSG_OOB},
}
