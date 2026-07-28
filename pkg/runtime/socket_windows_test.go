//go:build windows

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// newSocket builds a bare _socket.socket via the module class.
func newSocket(t *testing.T, af, sk int64) objects.Object {
	t.Helper()
	mo, err := ImportModule("_socket")
	if err != nil {
		t.Fatalf("import _socket: %v", err)
	}
	cls, err := objects.LoadAttr(mo, "socket")
	if err != nil {
		t.Fatalf("_socket.socket: %v", err)
	}
	s, err := objects.Call(cls, []objects.Object{objects.NewInt(af), objects.NewInt(sk)})
	if err != nil {
		t.Fatalf("socket(): %v", err)
	}
	return s
}

func callMethod(t *testing.T, o objects.Object, name string, args ...objects.Object) objects.Object {
	t.Helper()
	r, err := objects.CallMethod(o, name, args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

// TestSocketLoopback runs a TCP loopback round-trip through the winsock backend,
// exercising the ws2_32 accept path and the ReadFile/WriteFile send and recv.
func TestSocketLoopback(t *testing.T) {
	af := int64(2)     // AF_INET
	stream := int64(1) // SOCK_STREAM
	srv := newSocket(t, af, stream)
	cli := newSocket(t, af, stream)

	mo, _ := ImportModule("_socket")
	solSocket, _ := objects.LoadAttr(mo, "SOL_SOCKET")
	soReuse, _ := objects.LoadAttr(mo, "SO_REUSEADDR")
	callMethod(t, srv, "setsockopt", solSocket, soReuse, objects.NewInt(1))

	addr := objects.NewTuple([]objects.Object{objects.NewStr("127.0.0.1"), objects.NewInt(0)})
	callMethod(t, srv, "bind", addr)
	callMethod(t, srv, "listen", objects.NewInt(1))
	bound := callMethod(t, srv, "getsockname")
	callMethod(t, cli, "connect", bound)

	pair := callMethod(t, srv, "accept")
	conn, err := objects.GetItem(pair, objects.NewInt(0))
	if err != nil {
		t.Fatalf("accept[0]: %v", err)
	}

	callMethod(t, cli, "sendall", objects.NewBytes([]byte("ping")))
	got := callMethod(t, conn, "recv", objects.NewInt(16))
	if b, _ := objects.AsBytes(got); string(b) != "ping" {
		t.Errorf("recv = %q, want ping", b)
	}

	if n, _ := objects.AsInt(callMethod(t, cli, "fileno")); n < 0 {
		t.Errorf("open fileno = %d, want >= 0", n)
	}
	callMethod(t, cli, "close")
	if n, _ := objects.AsInt(callMethod(t, cli, "fileno")); n != -1 {
		t.Errorf("closed fileno = %d, want -1", n)
	}
	if _, err := objects.CallMethod(cli, "recv", []objects.Object{objects.NewInt(1)}); err == nil {
		t.Errorf("recv after close did not raise")
	}
	callMethod(t, conn, "close")
	callMethod(t, srv, "close")
}

// TestSocketUDPLoopback exercises the ws2_32 sendto and recvfrom paths, the two
// datagram calls Go's windows syscall leaves as EWINDOWS stubs.
func TestSocketUDPLoopback(t *testing.T) {
	af := int64(2)    // AF_INET
	dgram := int64(2) // SOCK_DGRAM
	srv := newSocket(t, af, dgram)
	cli := newSocket(t, af, dgram)
	defer objects.CallMethod(srv, "close", nil)
	defer objects.CallMethod(cli, "close", nil)

	addr := objects.NewTuple([]objects.Object{objects.NewStr("127.0.0.1"), objects.NewInt(0)})
	callMethod(t, srv, "bind", addr)
	bound := callMethod(t, srv, "getsockname")

	callMethod(t, cli, "sendto", objects.NewBytes([]byte("pong")), bound)
	pair := callMethod(t, srv, "recvfrom", objects.NewInt(16))
	data, err := objects.GetItem(pair, objects.NewInt(0))
	if err != nil {
		t.Fatalf("recvfrom[0]: %v", err)
	}
	if b, _ := objects.AsBytes(data); string(b) != "pong" {
		t.Errorf("recvfrom = %q, want pong", b)
	}
}

// TestSocketWindowsConstants pins the winsock-specific constant surface: no
// AF_UNIX, and the winsock SO_* option codes rather than the linux ones.
func TestSocketWindowsConstants(t *testing.T) {
	mo, err := ImportModule("_socket")
	if err != nil {
		t.Fatalf("import _socket: %v", err)
	}
	if _, err := objects.LoadAttr(mo, "AF_UNIX"); err == nil {
		t.Errorf("AF_UNIX present, but Windows omits it")
	}
	want := map[string]int64{
		"AF_INET":    2,
		"AF_INET6":   23,
		"SOL_SOCKET": 65535,
		"SO_ERROR":   4103,
		"SO_TYPE":    4104,
		"SO_RCVBUF":  4098,
		"SO_SNDBUF":  4097,
	}
	for name, val := range want {
		o, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n, _ := objects.AsInt(o); n != val {
			t.Errorf("%s = %d, want %d", name, n, val)
		}
	}
}
