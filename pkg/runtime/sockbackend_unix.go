//go:build !windows

package runtime

import "syscall"

// The unix _socket backend: every sock* call is a thin wrapper over the matching
// syscall function, which on unix takes and returns an int descriptor and a
// syscall.Sockaddr. Windows differs enough (SOCKET handles, and accept/recvfrom/
// sendto missing from Go's syscall) that it carries its own backend in
// sockbackend_windows.go.

// sockStartup is a no-op on unix; winsock needs WSAStartup, so the Windows
// backend does that work at first use instead.
func sockStartup() error { return nil }

func sockSocket(family, socktype, proto int) (int, error) {
	return syscall.Socket(family, socktype, proto)
}

func sockClose(fd int) error { return syscall.Close(fd) }

func sockConnect(fd int, sa syscall.Sockaddr) error { return syscall.Connect(fd, sa) }

func sockBind(fd int, sa syscall.Sockaddr) error { return syscall.Bind(fd, sa) }

func sockListen(fd, backlog int) error { return syscall.Listen(fd, backlog) }

func sockAccept(fd int) (int, syscall.Sockaddr, error) { return syscall.Accept(fd) }

func sockSend(fd int, data []byte) (int, error) { return syscall.Write(fd, data) }

func sockSendto(fd int, data []byte, flags int, sa syscall.Sockaddr) error {
	return syscall.Sendto(fd, data, flags, sa)
}

func sockRecv(fd int, buf []byte) (int, error) { return syscall.Read(fd, buf) }

func sockRecvfrom(fd int, buf []byte, flags int) (int, syscall.Sockaddr, error) {
	return syscall.Recvfrom(fd, buf, flags)
}

func sockShutdown(fd, how int) error { return syscall.Shutdown(fd, how) }

func sockSetsockoptInt(fd, level, opt, val int) error {
	return syscall.SetsockoptInt(fd, level, opt, val)
}

func sockGetsockoptInt(fd, level, opt int) (int, error) {
	return syscall.GetsockoptInt(fd, level, opt)
}

func sockGetsockname(fd int) (syscall.Sockaddr, error) { return syscall.Getsockname(fd) }

func sockGetpeername(fd int) (syscall.Sockaddr, error) { return syscall.Getpeername(fd) }
