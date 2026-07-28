//go:build windows

package runtime

import (
	"sync"
	"syscall"
	"unsafe"
)

// The Windows _socket backend. Most winsock calls have working wrappers in Go's
// syscall (Socket, Bind, Connect, Listen, Shutdown, Getsockname, Getpeername and
// the sockopt pair), and a Windows socket() is an ordinary blocking handle, so
// ReadFile/WriteFile (syscall.Read/Write) serve recv and send. The three that
// Go's windows syscall leaves as EWINDOWS stubs, accept, recvfrom and sendto, go
// straight to ws2_32 here. A SOCKET is a UINT_PTR, so the int fd the rest of the
// runtime carries is widened to a syscall.Handle at this boundary and no further.
// This is the AF_INET path, matching addrToSockaddr; AF_INET6 and AF_UNIX are a
// later slice.

var (
	procAccept   = ws2_32.NewProc("accept")
	procRecvfrom = ws2_32.NewProc("recvfrom")
	procSendto   = ws2_32.NewProc("sendto")
	procSend     = ws2_32.NewProc("send")
	procRecv     = ws2_32.NewProc("recv")
)

// wsaStartupOnce runs WSAStartup exactly once. Winsock refuses every call until
// the process has started it, and syscall.Socket does not do it for us, so the
// first socket() triggers it. Any failure is remembered and returned from every
// sockSocket so the caller raises OSError the way CPython does.
var (
	wsaStartupOnce sync.Once
	wsaStartupErr  error
)

func sockStartup() error {
	wsaStartupOnce.Do(func() {
		var data syscall.WSAData
		// Request Winsock 2.2, the version CPython asks for.
		wsaStartupErr = syscall.WSAStartup(uint32(0x202), &data)
	})
	return wsaStartupErr
}

func sockSocket(family, socktype, proto int) (int, error) {
	if err := sockStartup(); err != nil {
		return -1, err
	}
	h, err := syscall.Socket(family, socktype, proto)
	if err != nil {
		return -1, err
	}
	return int(h), nil
}

func sockClose(fd int) error { return syscall.Closesocket(syscall.Handle(fd)) }

func sockConnect(fd int, sa syscall.Sockaddr) error {
	return syscall.Connect(syscall.Handle(fd), sa)
}

func sockBind(fd int, sa syscall.Sockaddr) error {
	return syscall.Bind(syscall.Handle(fd), sa)
}

func sockListen(fd, backlog int) error {
	return syscall.Listen(syscall.Handle(fd), backlog)
}

func sockShutdown(fd, how int) error {
	return syscall.Shutdown(syscall.Handle(fd), how)
}

func sockSetsockoptInt(fd, level, opt, val int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), level, opt, val)
}

func sockGetsockoptInt(fd, level, opt int) (int, error) {
	return syscall.GetsockoptInt(syscall.Handle(fd), level, opt)
}

func sockGetsockname(fd int) (syscall.Sockaddr, error) {
	return syscall.Getsockname(syscall.Handle(fd))
}

func sockGetpeername(fd int) (syscall.Sockaddr, error) {
	return syscall.Getpeername(syscall.Handle(fd))
}

// send and recv go through ws2_32 rather than ReadFile/WriteFile: WriteFile on a
// winsock handle reports ERROR_INVALID_PARAMETER (87), so the socket calls
// themselves are the reliable path.

func sockSend(fd int, data []byte) (int, error) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	r0, _, _ := procSend.Call(
		uintptr(fd),
		uintptr(p),
		uintptr(len(data)),
		0,
	)
	if int32(r0) == -1 { // SOCKET_ERROR
		return 0, wsaErr()
	}
	return int(r0), nil
}

func sockRecv(fd int, buf []byte) (int, error) {
	var p unsafe.Pointer
	if len(buf) > 0 {
		p = unsafe.Pointer(&buf[0])
	}
	r0, _, _ := procRecv.Call(
		uintptr(fd),
		uintptr(p),
		uintptr(len(buf)),
		0,
	)
	if int32(r0) == -1 { // SOCKET_ERROR
		return 0, wsaErr()
	}
	return int(r0), nil
}

// rawSockaddrIn is a winsock sockaddr_in: family (host order), port (network
// order), the four address bytes, then eight zero pad bytes, sixteen bytes total.
type rawSockaddrIn struct {
	family uint16
	port   uint16
	addr   [4]byte
	zero   [8]byte
}

// marshalInet4 lays a syscall.SockaddrInet4 out as a sockaddr_in for sendto. The
// port goes network order (big endian); the family is written host order, which
// is little endian on amd64.
func marshalInet4(sa4 *syscall.SockaddrInet4) rawSockaddrIn {
	return rawSockaddrIn{
		family: uint16(syscall.AF_INET),
		port:   uint16(sa4.Port>>8) | uint16(sa4.Port&0xff)<<8,
		addr:   sa4.Addr,
	}
}

// parseInet4 reads a sockaddr_in written by accept or recvfrom back into a
// syscall.SockaddrInet4, undoing the network-order port.
func parseInet4(raw *rawSockaddrIn) *syscall.SockaddrInet4 {
	sa := &syscall.SockaddrInet4{Port: int(raw.port>>8) | int(raw.port&0xff)<<8}
	sa.Addr = raw.addr
	return sa
}

func wsaErr() error {
	code, _, _ := procWSAGetLastError.Call()
	return syscall.Errno(uint32(code))
}

func sockAccept(fd int) (int, syscall.Sockaddr, error) {
	var raw rawSockaddrIn
	salen := int32(unsafe.Sizeof(raw))
	r0, _, _ := procAccept.Call(
		uintptr(fd),
		uintptr(unsafe.Pointer(&raw)),
		uintptr(unsafe.Pointer(&salen)),
	)
	if int(r0) == -1 { // INVALID_SOCKET
		return -1, nil, wsaErr()
	}
	return int(r0), parseInet4(&raw), nil
}

func sockRecvfrom(fd int, buf []byte, flags int) (int, syscall.Sockaddr, error) {
	var raw rawSockaddrIn
	salen := int32(unsafe.Sizeof(raw))
	var p unsafe.Pointer
	if len(buf) > 0 {
		p = unsafe.Pointer(&buf[0])
	}
	r0, _, _ := procRecvfrom.Call(
		uintptr(fd),
		uintptr(p),
		uintptr(len(buf)),
		uintptr(flags),
		uintptr(unsafe.Pointer(&raw)),
		uintptr(unsafe.Pointer(&salen)),
	)
	if int32(r0) == -1 { // SOCKET_ERROR
		return 0, nil, wsaErr()
	}
	return int(r0), parseInet4(&raw), nil
}

func sockSendto(fd int, data []byte, flags int, sa syscall.Sockaddr) error {
	sa4, ok := sa.(*syscall.SockaddrInet4)
	if !ok {
		return syscall.EAFNOSUPPORT
	}
	raw := marshalInet4(sa4)
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	r0, _, _ := procSendto.Call(
		uintptr(fd),
		uintptr(p),
		uintptr(len(data)),
		uintptr(flags),
		uintptr(unsafe.Pointer(&raw)),
		uintptr(unsafe.Sizeof(raw)),
	)
	if int32(r0) == -1 { // SOCKET_ERROR
		return wsaErr()
	}
	return nil
}
