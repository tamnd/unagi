package runtime

import (
	"net"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// The _socket.socket type. CPython's low-level socket is a C type the public
// socket.py subclasses; here it is an ordinary class whose methods are Go
// functions over a per-GOOS socket backend, so an instance holds a real file
// descriptor and fileno(), connect(), bind(), listen(), accept() and the
// send/recv family behave the way the C socket does. socket.py subclasses this
// class and calls _socket.socket.__init__(self, ...) explicitly, which the
// unbound method read off the class supports.
//
// The native state lives in a socketHandle stashed on the instance under a
// private attribute, so the same methods serve a bare _socket.socket and a
// socket.socket subclass instance. The raw calls go through the sock* functions,
// implemented over syscall on unix (sockbackend_unix.go) and over ws2_32 on
// Windows (sockbackend_windows.go), where syscall's Accept/Recvfrom/Sendto are
// unimplemented stubs. This is the AF_INET path; the AF_INET6 and AF_UNIX
// address encodings are a later slice. The fd is kept as an int on every host;
// Windows widens it to a SOCKET handle only at the backend boundary.

const socketHandleAttr = "_unagi_sock"

// socketHandle is the native state of a socket: its descriptor, the family,
// type and protocol it was built with, and the timeout in seconds (a negative
// value means blocking, the None default; zero means non-blocking).
type socketHandle struct {
	fd       int
	family   int
	socktype int
	proto    int
	timeout  float64
	closed   bool
}

func (*socketHandle) TypeName() string { return "_socket_handle" }

// buildSocketClass builds the _socket.socket class. Its methods are self-bound
// builtins, so a read off an instance prepends self while a read off the class
// (socket.py's _socket.socket.__init__(self, ...)) stays unbound.
func buildSocketClass() (objects.Object, error) {
	// __slots__ = ["__dict__"] models a C extension type: it keeps a per-instance
	// __dict__ (where the native handle and the family/type/proto attributes live)
	// but advertises no __weakref__, so socket.py's subclass, which declares
	// __weakref__ in its own __slots__, is allowed to add weakref support the way
	// it does over the real C socket.
	names := []string{
		"__slots__",
		"family", "type", "proto",
		"__init__", "fileno", "close", "detach",
		"connect", "connect_ex", "bind", "listen", "_accept", "accept",
		"send", "sendall", "sendto", "recv", "recvfrom",
		"shutdown", "setsockopt", "getsockopt",
		"getsockname", "getpeername",
		"settimeout", "gettimeout", "setblocking", "getblocking",
	}
	vals := []objects.Object{
		objects.NewList([]objects.Object{objects.NewStr("__dict__")}),
		// family, type and proto are the read-only C-socket attributes, backed by
		// the native handle; socket.py's family and type properties read them
		// through super().family / super().type.
		objects.NewProperty(socketAttrGetter("family"), nil, nil),
		objects.NewProperty(socketAttrGetter("type"), nil, nil),
		objects.NewProperty(socketAttrGetter("proto"), nil, nil),
		objects.NewMethod("__init__", -1, socketInit),
		objects.NewMethod("fileno", 1, socketFileno),
		objects.NewMethod("close", 1, socketClose),
		objects.NewMethod("detach", 1, socketDetach),
		objects.NewMethod("connect", 2, socketConnect),
		objects.NewMethod("connect_ex", 2, socketConnectEx),
		objects.NewMethod("bind", 2, socketBind),
		objects.NewMethod("listen", -1, socketListen),
		objects.NewMethod("_accept", 1, socketAcceptRaw),
		objects.NewMethod("accept", 1, socketAccept),
		objects.NewMethod("send", -1, socketSend),
		objects.NewMethod("sendall", -1, socketSendall),
		objects.NewMethod("sendto", -1, socketSendto),
		objects.NewMethod("recv", -1, socketRecv),
		objects.NewMethod("recvfrom", -1, socketRecvfrom),
		objects.NewMethod("shutdown", 2, socketShutdown),
		objects.NewMethod("setsockopt", -1, socketSetsockopt),
		objects.NewMethod("getsockopt", -1, socketGetsockopt),
		objects.NewMethod("getsockname", 1, socketGetsockname),
		objects.NewMethod("getpeername", 1, socketGetpeername),
		objects.NewMethod("settimeout", 2, socketSettimeout),
		objects.NewMethod("gettimeout", 1, socketGettimeout),
		objects.NewMethod("setblocking", 2, socketSetblocking),
		objects.NewMethod("getblocking", 1, socketGetblocking),
	}
	cls, err := objects.NewClass("socket", "socket", nil, names, vals, nil, nil)
	if err != nil {
		return nil, err
	}
	socketClass = cls
	return cls, nil
}

// socketClass holds the built _socket.socket class so the bare accept() can
// construct a connection socket by adopting the accepted descriptor.
var socketClass objects.Object

// rawSocketOf reads the native handle off a socket instance without regard to
// whether it is closed; the family/type/proto attributes stay readable after
// close the way CPython's do.
func rawSocketOf(self objects.Object) (*socketHandle, error) {
	v, err := objects.LoadAttr(self, socketHandleAttr)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "not a socket object")
	}
	h, ok := v.(*socketHandle)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "not a socket object")
	}
	return h, nil
}

// socketOf reads the native handle for an operation that needs a live socket,
// raising the same "Bad file descriptor" OSError CPython gives once closed.
func socketOf(self objects.Object) (*socketHandle, error) {
	h, err := rawSocketOf(self)
	if err != nil {
		return nil, err
	}
	if h.closed {
		return nil, objects.Raise("OSError", "[Errno 9] Bad file descriptor")
	}
	return h, nil
}

// socketAttrGetter builds the read-only property getter for one of the C-socket
// attributes family, type and proto, reading the value from the native handle.
func socketAttrGetter(field string) objects.Object {
	return objects.NewFunc(field, 1, func(args []objects.Object) (objects.Object, error) {
		h, err := rawSocketOf(args[0])
		if err != nil {
			return nil, err
		}
		switch field {
		case "family":
			return objects.NewInt(int64(h.family)), nil
		case "type":
			return objects.NewInt(int64(h.socktype)), nil
		default:
			return objects.NewInt(int64(h.proto)), nil
		}
	})
}

// socketOSError wraps a Go syscall error as the OSError socket operations raise.
// The errno-tagged message is host specific, so a fixture asserts the class, not
// the text.
func socketOSError(err error) error {
	if errno, ok := err.(syscall.Errno); ok {
		return objects.Raise("OSError", "[Errno %d] %s", int(errno), errno.Error())
	}
	return objects.Raise("OSError", "%s", err.Error())
}

// socketInit is _socket.socket.__init__(self, family=AF_INET, type=SOCK_STREAM,
// proto=0, fileno=None). With no fileno it opens a fresh descriptor; with one it
// adopts an existing descriptor, the path socket.py's accept() takes to wrap the
// connection its _accept returned.
func socketInit(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 {
		return nil, objects.Raise(objects.TypeError, "__init__ needs self")
	}
	self := args[0]
	family, socktype, proto := int(syscall.AF_INET), int(syscall.SOCK_STREAM), 0
	rest := args[1:]
	if len(rest) >= 1 && rest[0] != objects.None {
		if n, ok := objects.AsInt(rest[0]); ok {
			family = int(n)
		}
	}
	if len(rest) >= 2 && rest[1] != objects.None {
		if n, ok := objects.AsInt(rest[1]); ok {
			socktype = int(n)
		}
	}
	if len(rest) >= 3 && rest[2] != objects.None {
		if n, ok := objects.AsInt(rest[2]); ok {
			proto = int(n)
		}
	}
	var fd int
	if len(rest) >= 4 && rest[3] != objects.None {
		n, ok := objects.AsInt(rest[3])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "fileno must be an integer")
		}
		fd = int(n)
	} else {
		f, err := sockSocket(family, socktype, proto)
		if err != nil {
			return nil, socketOSError(err)
		}
		fd = f
	}
	h := &socketHandle{fd: fd, family: family, socktype: socktype, proto: proto, timeout: -1}
	if err := objects.StoreAttr(self, socketHandleAttr, h); err != nil {
		return nil, err
	}
	// family, type and proto are exposed as read-only properties backed by this
	// handle, so nothing is stored for them here.
	return objects.None, nil
}

func socketFileno(args []objects.Object) (objects.Object, error) {
	h, err := rawSocketOf(args[0])
	if err != nil {
		return nil, err
	}
	// A closed or detached socket reports -1, the way CPython's fileno does once
	// the descriptor is gone.
	if h.closed {
		return objects.NewInt(-1), nil
	}
	return objects.NewInt(int64(h.fd)), nil
}

func socketClose(args []objects.Object) (objects.Object, error) {
	v, err := objects.LoadAttr(args[0], socketHandleAttr)
	if err != nil {
		return objects.None, nil
	}
	h, ok := v.(*socketHandle)
	if !ok || h.closed {
		return objects.None, nil
	}
	h.closed = true
	_ = sockClose(h.fd)
	return objects.None, nil
}

// socketDetach releases the descriptor from the socket without closing it and
// returns it, the way CPython's detach does.
func socketDetach(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	fd := h.fd
	h.closed = true
	h.fd = -1
	return objects.NewInt(int64(fd)), nil
}

// addrToSockaddr encodes an (host, port) address tuple for an AF_INET socket. An
// empty host is INADDR_ANY; a name is resolved to its first address.
func addrToSockaddr(family int, addr objects.Object) (syscall.Sockaddr, error) {
	if family != int(syscall.AF_INET) {
		return nil, objects.Raise("OSError", "address family %d not supported yet", family)
	}
	parts, err := objects.Unpack(addr, 2)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "AF_INET address must be a pair (host, port)")
	}
	host, ok := objects.AsStr(parts[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "AF_INET address host must be str")
	}
	port, ok := objects.AsInt(parts[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "AF_INET address port must be int")
	}
	sa := &syscall.SockaddrInet4{Port: int(port)}
	if host == "" {
		return sa, nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, lerr := net.LookupIP(host)
		for _, cand := range ips {
			if v4 := cand.To4(); v4 != nil {
				ip = cand
				break
			}
		}
		if ip == nil {
			if lerr != nil {
				return nil, objects.Raise("gaierror", "[Errno 8] nodename nor servname provided, or not known")
			}
			return nil, objects.Raise("gaierror", "no address associated with hostname")
		}
	}
	v4 := ip.To4()
	if v4 == nil {
		return nil, objects.Raise("OSError", "not an IPv4 address: %s", host)
	}
	copy(sa.Addr[:], v4)
	return sa, nil
}

// sockaddrToAddr renders a kernel sockaddr as the (host, port) tuple CPython
// returns from getsockname, getpeername and accept.
func sockaddrToAddr(sa syscall.Sockaddr) objects.Object {
	if in4, ok := sa.(*syscall.SockaddrInet4); ok {
		ip := net.IPv4(in4.Addr[0], in4.Addr[1], in4.Addr[2], in4.Addr[3]).String()
		return objects.NewTuple([]objects.Object{objects.NewStr(ip), objects.NewInt(int64(in4.Port))})
	}
	return objects.None
}

func socketConnect(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	sa, err := addrToSockaddr(h.family, args[1])
	if err != nil {
		return nil, err
	}
	if err := sockConnect(h.fd, sa); err != nil {
		return nil, socketOSError(err)
	}
	return objects.None, nil
}

// socketConnectEx returns the connection errno instead of raising, the way
// connect_ex does; zero means success.
func socketConnectEx(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	sa, err := addrToSockaddr(h.family, args[1])
	if err != nil {
		return nil, err
	}
	if cerr := sockConnect(h.fd, sa); cerr != nil {
		if errno, ok := cerr.(syscall.Errno); ok {
			return objects.NewInt(int64(errno)), nil
		}
		return nil, socketOSError(cerr)
	}
	return objects.NewInt(0), nil
}

func socketBind(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	sa, err := addrToSockaddr(h.family, args[1])
	if err != nil {
		return nil, err
	}
	if err := sockBind(h.fd, sa); err != nil {
		return nil, socketOSError(err)
	}
	return objects.None, nil
}

func socketListen(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	backlog := 128
	if len(args) >= 2 {
		if n, ok := objects.AsInt(args[1]); ok {
			backlog = int(n)
		}
	}
	if err := sockListen(h.fd, backlog); err != nil {
		return nil, socketOSError(err)
	}
	return objects.None, nil
}

// socketAcceptRaw is _socket.socket._accept: it returns (fd, addr) for the next
// pending connection, which socket.py wraps into a new socket object.
func socketAcceptRaw(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	nfd, sa, aerr := sockAccept(h.fd)
	if aerr != nil {
		return nil, socketOSError(aerr)
	}
	return objects.NewTuple([]objects.Object{objects.NewInt(int64(nfd)), sockaddrToAddr(sa)}), nil
}

// socketAccept is the direct _socket.socket.accept, building the connection
// socket itself; socket.py overrides it, so this serves bare _socket use.
func socketAccept(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	nfd, sa, aerr := sockAccept(h.fd)
	if aerr != nil {
		return nil, socketOSError(aerr)
	}
	// Build a connection socket by adopting the accepted descriptor: call the
	// class with (family, type, proto, fileno) so __init__ takes the fileno path.
	conn, cerr := objects.Call(socketClass, []objects.Object{
		objects.NewInt(int64(h.family)),
		objects.NewInt(int64(h.socktype)),
		objects.NewInt(int64(h.proto)),
		objects.NewInt(int64(nfd)),
	})
	if cerr != nil {
		return nil, cerr
	}
	return objects.NewTuple([]objects.Object{conn, sockaddrToAddr(sa)}), nil
}

func socketSend(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	data, ok := objects.AsBytesLike(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not %s", args[1].TypeName())
	}
	n, werr := sockSend(h.fd, data)
	if werr != nil {
		return nil, socketOSError(werr)
	}
	return objects.NewInt(int64(n)), nil
}

func socketSendall(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	data, ok := objects.AsBytesLike(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not %s", args[1].TypeName())
	}
	for len(data) > 0 {
		n, werr := sockSend(h.fd, data)
		if werr != nil {
			return nil, socketOSError(werr)
		}
		data = data[n:]
	}
	return objects.None, nil
}

func socketSendto(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	if len(args) < 3 {
		return nil, objects.Raise(objects.TypeError, "sendto() takes at least 3 arguments")
	}
	data, ok := objects.AsBytesLike(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required")
	}
	sa, aerr := addrToSockaddr(h.family, args[len(args)-1])
	if aerr != nil {
		return nil, aerr
	}
	if serr := sockSendto(h.fd, data, 0, sa); serr != nil {
		return nil, socketOSError(serr)
	}
	return objects.NewInt(int64(len(data))), nil
}

func socketRecv(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	bufsize := 0
	if len(args) >= 2 {
		if n, ok := objects.AsInt(args[1]); ok {
			bufsize = int(n)
		}
	}
	buf := make([]byte, bufsize)
	n, rerr := sockRecv(h.fd, buf)
	if rerr != nil {
		return nil, socketOSError(rerr)
	}
	return objects.NewBytes(buf[:n]), nil
}

func socketRecvfrom(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	bufsize := 0
	if len(args) >= 2 {
		if n, ok := objects.AsInt(args[1]); ok {
			bufsize = int(n)
		}
	}
	buf := make([]byte, bufsize)
	n, from, rerr := sockRecvfrom(h.fd, buf, 0)
	if rerr != nil {
		return nil, socketOSError(rerr)
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(buf[:n]), sockaddrToAddr(from)}), nil
}

func socketShutdown(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	how, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "how must be an integer")
	}
	if serr := sockShutdown(h.fd, int(how)); serr != nil {
		return nil, socketOSError(serr)
	}
	return objects.None, nil
}

func socketSetsockopt(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	if len(args) < 4 {
		return nil, objects.Raise(objects.TypeError, "setsockopt() takes at least 4 arguments")
	}
	level, _ := objects.AsInt(args[1])
	opt, _ := objects.AsInt(args[2])
	if val, ok := objects.AsInt(args[3]); ok {
		if serr := sockSetsockoptInt(h.fd, int(level), int(opt), int(val)); serr != nil {
			return nil, socketOSError(serr)
		}
		return objects.None, nil
	}
	return nil, objects.Raise(objects.TypeError, "setsockopt value must be an integer for now")
}

func socketGetsockopt(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	if len(args) < 3 {
		return nil, objects.Raise(objects.TypeError, "getsockopt() takes at least 3 arguments")
	}
	level, _ := objects.AsInt(args[1])
	opt, _ := objects.AsInt(args[2])
	v, gerr := sockGetsockoptInt(h.fd, int(level), int(opt))
	if gerr != nil {
		return nil, socketOSError(gerr)
	}
	return objects.NewInt(int64(v)), nil
}

func socketGetsockname(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	sa, gerr := sockGetsockname(h.fd)
	if gerr != nil {
		return nil, socketOSError(gerr)
	}
	return sockaddrToAddr(sa), nil
}

func socketGetpeername(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	sa, gerr := sockGetpeername(h.fd)
	if gerr != nil {
		return nil, socketOSError(gerr)
	}
	return sockaddrToAddr(sa), nil
}

func socketSettimeout(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	if args[1] == objects.None {
		h.timeout = -1
		return objects.None, nil
	}
	f, ok := objects.AsFloat(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a float is required")
	}
	if f < 0 {
		return nil, objects.Raise(objects.ValueError, "Timeout value out of range")
	}
	h.timeout = f
	return objects.None, nil
}

func socketGettimeout(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	if h.timeout < 0 {
		return objects.None, nil
	}
	return objects.NewFloat(h.timeout), nil
}

func socketSetblocking(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	truthy, terr := objects.TruthOf(args[1])
	if terr != nil {
		return nil, terr
	}
	if truthy {
		h.timeout = -1
	} else {
		h.timeout = 0
	}
	return objects.None, nil
}

func socketGetblocking(args []objects.Object) (objects.Object, error) {
	h, err := socketOf(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewBool(h.timeout != 0), nil
}
