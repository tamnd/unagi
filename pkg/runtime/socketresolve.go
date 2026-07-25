package runtime

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// The address-family and socket-type numbers the resolution code branches on,
// taken from syscall so they match the build platform the way the module
// constants do.
const (
	afInet     = syscall.AF_INET
	afInet6    = syscall.AF_INET6
	sockStream = syscall.SOCK_STREAM
	sockDgram  = syscall.SOCK_DGRAM
)

// Name resolution for _socket: getaddrinfo, gethostname, the gethostby* family,
// getnameinfo and the service/protocol lookups. socket.py re-exports these
// through `from _socket import *` and wraps getaddrinfo to convert the family
// and type ints to IntEnum members. The lookups run over Go's net package and
// os.Hostname; the AI_/NI_ flag constants come from the platform-tagged
// socketResolveConsts. This is the AF_INET path (with AF_INET6 results when a
// name resolves to a v6 address); AF_UNIX name resolution has no meaning.

// registerSocketResolve installs the name-resolution surface on the _socket
// module.
func registerSocketResolve(set func(string, objects.Object) error) error {
	for name, val := range socketResolveConsts {
		if err := set(name, objects.NewInt(val)); err != nil {
			return err
		}
	}
	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"getaddrinfo", socketGetaddrinfo},
		{"gethostname", socketGethostname},
		{"gethostbyname", socketGethostbyname},
		{"gethostbyname_ex", socketGethostbynameEx},
		{"gethostbyaddr", socketGethostbyaddr},
		{"getnameinfo", socketGetnameinfo},
		{"getservbyname", socketGetservbyname},
		{"getservbyport", socketGetservbyport},
		{"getprotobyname", socketGetprotobyname},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// coerceInt reads an integer argument that may be an IntEnum member. socket.py
// passes AddressFamily and SocketKind members (int subclasses) straight through
// to _socket.getaddrinfo, and a plain AsInt does not unwrap them. AsIntValue
// reaches through the int-subclass payload the way int() does, where the
// __index__ dunder the members do not expose would not.
func coerceInt(o objects.Object) (int64, bool) {
	return objects.AsIntValue(o)
}

// protoByName maps the protocol names getprotobyname answers to their numbers,
// the common subset of /etc/protocols every platform shares.
var protoByName = map[string]int64{
	"ip": 0, "icmp": 1, "igmp": 2, "tcp": 6, "udp": 17,
	"ipv6": 41, "ipv6-icmp": 58, "raw": 255,
}

// protoForType returns the default protocol number for a socket type, the way
// getaddrinfo fills proto from a TCP or UDP socktype.
func protoForType(socktype int64) int64 {
	switch socktype {
	case int64(sockStream):
		return 6 // IPPROTO_TCP
	case int64(sockDgram):
		return 17 // IPPROTO_UDP
	default:
		return 0
	}
}

// resolvePort turns the port argument of getaddrinfo into a number: an int is
// taken as is, a numeric string is parsed, and a service name is looked up for
// the protocol the socktype implies. None is port zero.
func resolvePort(port objects.Object, socktype int64) (int, error) {
	if port == objects.None {
		return 0, nil
	}
	if n, ok := objects.AsInt(port); ok {
		return int(n), nil
	}
	s, ok := asServiceString(port)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "getaddrinfo() argument 2 must be integer or string")
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	proto := "tcp"
	if socktype == int64(sockDgram) {
		proto = "udp"
	}
	p, err := net.LookupPort(proto, s)
	if err != nil {
		return 0, objects.Raise("OSError", "service/proto not found")
	}
	return p, nil
}

// asServiceString reads a str or bytes service argument as a Go string.
func asServiceString(o objects.Object) (string, bool) {
	if s, ok := objects.AsStr(o); ok {
		return s, true
	}
	if b, ok := objects.AsBytesLike(o); ok {
		return string(b), true
	}
	return "", false
}

// socketGetaddrinfo is getaddrinfo(host, port, family=0, type=0, proto=0,
// flags=0): it resolves host and port to a list of (family, type, proto,
// canonname, sockaddr) tuples with plain-int family and type, which socket.py
// converts to IntEnum members. A numeric host skips DNS; a name is resolved
// through net.LookupIP and filtered by the requested family.
func socketGetaddrinfo(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "getaddrinfo() takes at least 2 arguments (%d given)", len(args))
	}
	arg := func(i int) objects.Object {
		if i < len(args) {
			return args[i]
		}
		return objects.None
	}
	intArg := func(i int) int64 {
		if v, ok := coerceInt(arg(i)); ok {
			return v
		}
		return 0
	}
	family := intArg(2)
	socktype := intArg(3)
	proto := intArg(4)
	flags := intArg(5)

	// A missing type defaults to a stream socket, so a bare getaddrinfo(host,
	// port) resolves the TCP endpoint create_connection wants.
	if socktype == 0 {
		socktype = int64(sockStream)
	}
	if proto == 0 {
		proto = protoForType(socktype)
	}

	host := arg(0)
	var ips []net.IP
	if host == objects.None {
		// No host: the wildcard address with AI_PASSIVE (for a listening socket),
		// the loopback without it, mirroring getaddrinfo's passive rule.
		passive := flags&socketResolveConsts["AI_PASSIVE"] != 0
		if family == int64(afInet6) {
			if passive {
				ips = []net.IP{net.IPv6zero}
			} else {
				ips = []net.IP{net.IPv6loopback}
			}
		} else if passive {
			ips = []net.IP{net.IPv4zero}
		} else {
			ips = []net.IP{net.IPv4(127, 0, 0, 1)}
		}
	} else {
		hs, ok := asServiceString(host)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "getaddrinfo() argument 1 must be string or None")
		}
		if ip := net.ParseIP(hs); ip != nil {
			ips = []net.IP{ip}
		} else {
			resolved, err := net.LookupIP(hs)
			if err != nil {
				return nil, objects.Raise("gaierror", "[Errno 8] nodename nor servname provided, or not known")
			}
			ips = resolved
		}
	}

	port, err := resolvePort(arg(1), socktype)
	if err != nil {
		return nil, err
	}

	var results []objects.Object
	for _, ip := range ips {
		v4 := ip.To4()
		af := int64(afInet)
		var sa objects.Object
		if v4 != nil {
			if family == int64(afInet6) {
				continue
			}
			sa = objects.NewTuple([]objects.Object{objects.NewStr(v4.String()), objects.NewInt(int64(port))})
		} else {
			if family == int64(afInet) {
				continue
			}
			af = int64(afInet6)
			// The v6 sockaddr is a 4-tuple (host, port, flowinfo, scope_id).
			sa = objects.NewTuple([]objects.Object{
				objects.NewStr(ip.String()), objects.NewInt(int64(port)),
				objects.NewInt(0), objects.NewInt(0),
			})
		}
		results = append(results, objects.NewTuple([]objects.Object{
			objects.NewInt(af),
			objects.NewInt(socktype),
			objects.NewInt(proto),
			objects.NewStr(""),
			sa,
		}))
	}
	if len(results) == 0 {
		return nil, objects.Raise("gaierror", "[Errno 8] nodename nor servname provided, or not known")
	}
	return objects.NewList(results), nil
}

func socketGethostname(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "gethostname() takes no arguments (%d given)", len(args))
	}
	name, err := os.Hostname()
	if err != nil {
		return nil, socketOSError(err)
	}
	return objects.NewStr(name), nil
}

// socketGethostbyname resolves a host name to a single IPv4 address string.
func socketGethostbyname(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "gethostbyname() takes exactly one argument (%d given)", len(args))
	}
	host, ok := asServiceString(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "gethostbyname() argument 1 must be str, not %s", args[0].TypeName())
	}
	ip, err := firstIPv4(host)
	if err != nil {
		return nil, err
	}
	return objects.NewStr(ip), nil
}

// socketGethostbynameEx returns (hostname, aliaslist, ipaddrlist) with the v4
// addresses, the shape gethostbyname_ex gives. The alias list is empty, which
// Go's resolver does not surface.
func socketGethostbynameEx(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "gethostbyname_ex() takes exactly one argument (%d given)", len(args))
	}
	host, ok := asServiceString(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "gethostbyname_ex() argument 1 must be str, not %s", args[0].TypeName())
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, objects.Raise("gaierror", "[Errno 8] nodename nor servname provided, or not known")
	}
	var addrs []objects.Object
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			addrs = append(addrs, objects.NewStr(v4.String()))
		}
	}
	if len(addrs) == 0 {
		return nil, objects.Raise("gaierror", "[Errno 8] nodename nor servname provided, or not known")
	}
	return objects.NewTuple([]objects.Object{
		objects.NewStr(host),
		objects.NewList(nil),
		objects.NewList(addrs),
	}), nil
}

// socketGethostbyaddr returns (hostname, aliaslist, ipaddrlist) for an address,
// reverse-resolving the name the way gethostbyaddr does.
func socketGethostbyaddr(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "gethostbyaddr() takes exactly one argument (%d given)", len(args))
	}
	addr, ok := asServiceString(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "gethostbyaddr() argument 1 must be str, not %s", args[0].TypeName())
	}
	names, err := net.LookupAddr(addr)
	if err != nil || len(names) == 0 {
		return nil, objects.Raise("herror", "[Errno 1] Unknown host")
	}
	name := strings.TrimSuffix(names[0], ".")
	return objects.NewTuple([]objects.Object{
		objects.NewStr(name),
		objects.NewList(nil),
		objects.NewList([]objects.Object{objects.NewStr(addr)}),
	}), nil
}

// socketGetnameinfo turns a sockaddr into (host, service). NI_NUMERICHOST keeps
// the numeric address and NI_NUMERICSERV the numeric port; otherwise the host
// is reverse-resolved and the port mapped to a service name.
func socketGetnameinfo(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "getnameinfo() takes exactly 2 arguments (%d given)", len(args))
	}
	parts, err := objects.Unpack(args[0], 2)
	if err != nil {
		// A v6 sockaddr is a 4-tuple; take its first two members.
		if four, e4 := objects.Unpack(args[0], 4); e4 == nil {
			parts = four[:2]
		} else {
			return nil, objects.Raise(objects.TypeError, "getnameinfo() argument 1 must be a tuple")
		}
	}
	host, ok := objects.AsStr(parts[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getnameinfo() host must be str")
	}
	port, ok := objects.AsInt(parts[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getnameinfo() port must be int")
	}
	flags, _ := objects.AsInt(args[1])

	hostOut := host
	if flags&socketResolveConsts["NI_NUMERICHOST"] == 0 {
		if names, err := net.LookupAddr(host); err == nil && len(names) > 0 {
			hostOut = strings.TrimSuffix(names[0], ".")
		}
	}
	servOut := strconv.FormatInt(port, 10)
	if flags&socketResolveConsts["NI_NUMERICSERV"] == 0 {
		if name, ok := serviceByPort(int(port), "tcp"); ok {
			servOut = name
		}
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(hostOut), objects.NewStr(servOut)}), nil
}

// socketGetservbyname looks a service name up to its port for a protocol,
// defaulting across tcp and udp when no protocol is given.
func socketGetservbyname(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "getservbyname() takes 1 or 2 arguments (%d given)", len(args))
	}
	service, ok := asServiceString(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getservbyname() argument 1 must be str")
	}
	protos := []string{"tcp", "udp"}
	if len(args) == 2 && args[1] != objects.None {
		p, ok := asServiceString(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "getservbyname() argument 2 must be str")
		}
		protos = []string{p}
	}
	for _, proto := range protos {
		if n, err := net.LookupPort(proto, service); err == nil {
			return objects.NewInt(int64(n)), nil
		}
	}
	return nil, objects.Raise("OSError", "service/proto not found")
}

// socketGetservbyport maps a port back to its service name for a protocol,
// reading /etc/services the way getservbyport does.
func socketGetservbyport(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "getservbyport() takes 1 or 2 arguments (%d given)", len(args))
	}
	port, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getservbyport() argument 1 must be int")
	}
	if port < 0 || port > 65535 {
		return nil, objects.Raise("OverflowError", "getservbyport: port must be 0-65535.")
	}
	proto := "tcp"
	if len(args) == 2 && args[1] != objects.None {
		p, ok := asServiceString(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "getservbyport() argument 2 must be str")
		}
		proto = p
	}
	if name, ok := serviceByPort(int(port), proto); ok {
		return objects.NewStr(name), nil
	}
	return nil, objects.Raise("OSError", "port/proto not found")
}

// socketGetprotobyname returns the protocol number for a name, from the common
// /etc/protocols subset.
func socketGetprotobyname(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "getprotobyname() takes exactly one argument (%d given)", len(args))
	}
	name, ok := asServiceString(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getprotobyname() argument 1 must be str")
	}
	if n, ok := protoByName[strings.ToLower(name)]; ok {
		return objects.NewInt(n), nil
	}
	return nil, objects.Raise("OSError", "protocol not found")
}

// firstIPv4 resolves a host to its first IPv4 address string, taking a numeric
// address directly.
func firstIPv4(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", objects.Raise("gaierror", "[Errno 8] nodename nor servname provided, or not known")
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", objects.Raise("gaierror", "no address associated with hostname")
}

// serviceByPort scans /etc/services for the name of a port and protocol. Go's
// net package resolves a name to a port but not the reverse, so getservbyport
// reads the file directly, the source getservbyport itself consults.
func serviceByPort(port int, proto string) (string, bool) {
	f, err := os.Open("/etc/services")
	if err != nil {
		return "", false
	}
	defer f.Close()
	want := strconv.Itoa(port) + "/" + proto
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == want {
			return fields[0], true
		}
	}
	return "", false
}
