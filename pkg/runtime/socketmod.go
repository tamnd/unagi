package runtime

import (
	"encoding/binary"
	"net"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// _socket is a built-in module: CPython implements the low-level socket surface
// in C, so the runtime provides it in Go, with the public socket.py layered on
// top. This is the deterministic half: the address-family, socket-type,
// protocol and option constants (per-GOOS in socketModuleConsts so each value
// matches the platform the program is built on, the way CPython's differ by
// platform, including Windows omitting AF_UNIX and using the winsock SO_* codes),
// the error hierarchy (error is OSError, gaierror and herror are its subclasses,
// timeout is TimeoutError), and the byte-order and address conversion helpers.
// The socket object and name resolution follow in socketobj.go and
// socketresolve.go, with the raw syscalls behind a per-GOOS backend.

// socketConst is one named integer the _socket module exposes. The set and the
// values differ by platform, so the table lives per-GOOS in socketmodconst_unix
// (from syscall) and socketmodconst_windows (winsock literals).
type socketConst struct {
	name string
	val  int64
}

func init() {
	moduleTable["_socket"] = &moduleEntry{builtin: true, exec: initSocket}
}

func initSocket(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}

	// The address-family, socket-type, protocol and option constants. The set and
	// values are per-GOOS (AF_INET6 is 30 on darwin, 10 on linux, 23 on Windows;
	// SOL_SOCKET is 0xffff on darwin and Windows, 1 on linux; Windows omits
	// AF_UNIX and gives SO_ERROR 0x1007), so socketModuleConsts carries the right
	// table for the build platform.
	for _, c := range socketModuleConsts {
		if err := set(c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}
	// The IANA well-known addresses are the same on every platform.
	for _, c := range []socketConst{
		{"INADDR_ANY", 0x00000000},
		{"INADDR_BROADCAST", 0xffffffff},
		{"INADDR_LOOPBACK", 0x7f000001},
		{"INADDR_NONE", 0xffffffff},
	} {
		if err := set(c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}
	// has_ipv6 reports whether the platform build supports IPv6; the runtime
	// always does.
	if err := set("has_ipv6", objects.True); err != nil {
		return err
	}

	// The error hierarchy. socket.error is an exact alias of OSError, and timeout
	// is an alias of TimeoutError, the way CPython retired both to aliases.
	osErr, ok := objects.ExcClassValue("OSError")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_socket: OSError class unavailable")
	}
	if err := set("error", osErr); err != nil {
		return err
	}
	timeoutErr, ok := objects.ExcClassValue("TimeoutError")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_socket: TimeoutError class unavailable")
	}
	if err := set("timeout", timeoutErr); err != nil {
		return err
	}
	// gaierror and herror are OSError subclasses getaddrinfo and gethostby* raise.
	for _, name := range []string{"gaierror", "herror"} {
		cls, err := objects.NewClass(name, "_socket."+name, []objects.Object{osErr}, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		if err := set(name, cls); err != nil {
			return err
		}
	}

	// The byte-order helpers convert 16- and 32-bit values between host and
	// network order. On a little-endian build they byte-swap; on a big-endian one
	// they are identity, which the encoding/binary native/big pairing expresses
	// without a platform branch.
	byteOrder := []struct {
		name  string
		width int
		toNet bool
	}{
		{"htons", 16, true},
		{"ntohs", 16, false},
		{"htonl", 32, true},
		{"ntohl", 32, false},
	}
	for _, b := range byteOrder {
		if err := set(b.name, objects.NewFunc(b.name, -1, func(args []objects.Object) (objects.Object, error) {
			return socketByteOrder(b.name, b.width, b.toNet, args)
		})); err != nil {
			return err
		}
	}

	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"inet_aton", socketInetAton},
		{"inet_ntoa", socketInetNtoa},
		{"inet_pton", socketInetPton},
		{"inet_ntop", socketInetNtop},
		{"getdefaulttimeout", socketGetDefaultTimeout},
		{"setdefaulttimeout", socketSetDefaultTimeout},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}

	// The socket type itself. socket.py subclasses it at module level, so it is
	// an ordinary class whose methods are Go functions over syscall, holding a
	// real descriptor per instance.
	sockCls, err := buildSocketClass()
	if err != nil {
		return err
	}
	if err := set("socket", sockCls); err != nil {
		return err
	}
	// SocketType is an alias of the socket class, the way CPython exposes it.
	if err := set("SocketType", sockCls); err != nil {
		return err
	}

	// Name resolution: getaddrinfo, gethostname, the gethostby* family,
	// getnameinfo and the service/protocol lookups, plus the AI_/NI_ flags.
	if err := registerSocketResolve(set); err != nil {
		return err
	}
	return nil
}

// socketDefaultTimeout holds the module-wide default timeout in seconds, None
// until set. A socket object created after a setdefaulttimeout call inherits it;
// the socket object is a later slice, so for now the pair just round-trips.
var socketDefaultTimeout objects.Object = objects.None

// socketByteOrder implements htons/ntohs/htonl/ntohl over a width. A negative
// argument is the ValueError CPython raises, and a value past the unsigned width
// is the OverflowError.
func socketByteOrder(name string, width int, toNet bool, args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "%s() takes exactly one argument (%d given)", name, len(args))
	}
	x, ok := objects.AsInt(args[0])
	if !ok {
		// AsInt fails either for a non-int or for an int too large to fit int64.
		// A positive int past the width is an overflow; anything else is a type
		// error. (An int smaller than -2**63 mis-reports as overflow rather than
		// the negative-int ValueError, a pathological case no address code hits.)
		if tn := args[0].TypeName(); tn == "int" || tn == "bool" {
			return nil, socketWidthOverflow(width)
		}
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	if x < 0 {
		return nil, objects.Raise(objects.ValueError, "Cannot convert negative int")
	}
	if width == 16 {
		if x > 0xffff {
			return nil, socketWidthOverflow(16)
		}
		var buf [2]byte
		if toNet {
			binary.BigEndian.PutUint16(buf[:], uint16(x))
			return objects.NewInt(int64(binary.NativeEndian.Uint16(buf[:]))), nil
		}
		binary.NativeEndian.PutUint16(buf[:], uint16(x))
		return objects.NewInt(int64(binary.BigEndian.Uint16(buf[:]))), nil
	}
	if x > 0xffffffff {
		return nil, socketWidthOverflow(32)
	}
	var buf [4]byte
	if toNet {
		binary.BigEndian.PutUint32(buf[:], uint32(x))
		return objects.NewInt(int64(binary.NativeEndian.Uint32(buf[:]))), nil
	}
	binary.NativeEndian.PutUint32(buf[:], uint32(x))
	return objects.NewInt(int64(binary.BigEndian.Uint32(buf[:]))), nil
}

func socketWidthOverflow(width int) error {
	if width == 16 {
		return objects.Raise(objects.OverflowError, "Python int too large for C uint16_t")
	}
	return objects.Raise(objects.OverflowError, "Python int too large for C uint32_t")
}

// socketInetAton packs a dotted-quad IPv4 string into four network-order bytes.
// It handles the ordinary a.b.c.d form; the legacy short and hex forms CPython's
// inet_aton also accepts are not reproduced, and a string it cannot parse as a
// dotted quad is the OSError CPython raises.
func socketInetAton(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "inet_aton() takes exactly one argument (%d given)", len(args))
	}
	s, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "inet_aton() argument 1 must be str, not %s", args[0].TypeName())
	}
	ip := net.ParseIP(s)
	v4 := ip.To4()
	if ip == nil || v4 == nil {
		return nil, objects.Raise(osErrorClass(), "illegal IP address string passed to inet_aton")
	}
	return objects.NewBytes([]byte{v4[0], v4[1], v4[2], v4[3]}), nil
}

// socketInetNtoa renders four packed network-order bytes as a dotted-quad string.
func socketInetNtoa(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "inet_ntoa() takes exactly one argument (%d given)", len(args))
	}
	b, ok := objects.AsBytesLike(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "inet_ntoa() argument 1 must be a bytes-like object, not %s", args[0].TypeName())
	}
	if len(b) != 4 {
		return nil, objects.Raise(osErrorClass(), "packed IP wrong length for inet_ntoa")
	}
	return objects.NewStr(net.IPv4(b[0], b[1], b[2], b[3]).String()), nil
}

// socketInetPton packs a string address of the given family into its network
// bytes: four for AF_INET, sixteen for AF_INET6. An unsupported family or an
// unparseable address is the OSError CPython raises.
func socketInetPton(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "inet_pton() takes exactly 2 arguments (%d given)", len(args))
	}
	af, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	s, ok := objects.AsStr(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "inet_pton() argument 2 must be str, not %s", args[1].TypeName())
	}
	switch af {
	case int64(syscall.AF_INET):
		ip := net.ParseIP(s)
		v4 := ip.To4()
		if ip == nil || v4 == nil {
			return nil, objects.Raise(osErrorClass(), "illegal IP address string passed to inet_pton")
		}
		return objects.NewBytes([]byte{v4[0], v4[1], v4[2], v4[3]}), nil
	case int64(syscall.AF_INET6):
		ip := net.ParseIP(s)
		if ip == nil || ip.To4() != nil {
			return nil, objects.Raise(osErrorClass(), "illegal IP address string passed to inet_pton")
		}
		v6 := ip.To16()
		return objects.NewBytes(append([]byte(nil), v6...)), nil
	default:
		return nil, objects.Raise(osErrorClass(), "Address family not supported by protocol family")
	}
}

// socketInetNtop renders packed network bytes of the given family as a string.
func socketInetNtop(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "inet_ntop() takes exactly 2 arguments (%d given)", len(args))
	}
	af, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	b, ok := objects.AsBytesLike(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "inet_ntop() argument 2 must be a bytes-like object, not %s", args[1].TypeName())
	}
	switch af {
	case int64(syscall.AF_INET):
		if len(b) != 4 {
			return nil, objects.Raise(objects.ValueError, "invalid length of packed IP address string")
		}
		return objects.NewStr(net.IPv4(b[0], b[1], b[2], b[3]).String()), nil
	case int64(syscall.AF_INET6):
		if len(b) != 16 {
			return nil, objects.Raise(objects.ValueError, "invalid length of packed IP address string")
		}
		return objects.NewStr(net.IP(append([]byte(nil), b...)).String()), nil
	default:
		return nil, objects.Raise(osErrorClass(), "unknown address family")
	}
}

// socketGetDefaultTimeout returns the module default timeout, None or a float.
func socketGetDefaultTimeout(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getdefaulttimeout() takes no arguments (%d given)", len(args))
	}
	return socketDefaultTimeout, nil
}

// socketSetDefaultTimeout sets the module default timeout from None or a
// non-negative real number.
func socketSetDefaultTimeout(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "setdefaulttimeout() takes exactly one argument (%d given)", len(args))
	}
	if args[0] == objects.None {
		socketDefaultTimeout = objects.None
		return objects.None, nil
	}
	f, ok := objects.AsFloat(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a float is required")
	}
	if f < 0 {
		return nil, objects.Raise(objects.ValueError, "Timeout value out of range")
	}
	socketDefaultTimeout = objects.NewFloat(f)
	return objects.None, nil
}

// osErrorClass names the OSError the address helpers raise; socket.error is an
// alias of it, so a program catching either name sees the same class.
func osErrorClass() string { return "OSError" }
