//go:build !windows

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

func TestSocketLoopback(t *testing.T) {
	af := int64(2)     // AF_INET
	stream := int64(1) // SOCK_STREAM
	srv := newSocket(t, af, stream)
	cli := newSocket(t, af, stream)

	call := func(o objects.Object, name string, args ...objects.Object) objects.Object {
		t.Helper()
		r, err := objects.CallMethod(o, name, args)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return r
	}

	// SO_REUSEADDR is 2 on linux and 4 on darwin; read it off the module rather
	// than hard-coding it.
	mo, _ := ImportModule("_socket")
	solSocket, _ := objects.LoadAttr(mo, "SOL_SOCKET")
	soReuse, _ := objects.LoadAttr(mo, "SO_REUSEADDR")
	call(srv, "setsockopt", solSocket, soReuse, objects.NewInt(1))

	addr := objects.NewTuple([]objects.Object{objects.NewStr("127.0.0.1"), objects.NewInt(0)})
	call(srv, "bind", addr)
	call(srv, "listen", objects.NewInt(1))
	bound := call(srv, "getsockname")
	call(cli, "connect", bound)

	pair := call(srv, "accept")
	conn, err := objects.GetItem(pair, objects.NewInt(0))
	if err != nil {
		t.Fatalf("accept[0]: %v", err)
	}

	call(cli, "sendall", objects.NewBytes([]byte("ping")))
	got := call(conn, "recv", objects.NewInt(16))
	if b, _ := objects.AsBytes(got); string(b) != "ping" {
		t.Errorf("recv = %q, want ping", b)
	}

	// fileno is live while open and -1 once closed.
	if n, _ := objects.AsInt(call(cli, "fileno")); n < 0 {
		t.Errorf("open fileno = %d, want >= 0", n)
	}
	call(cli, "close")
	if n, _ := objects.AsInt(call(cli, "fileno")); n != -1 {
		t.Errorf("closed fileno = %d, want -1", n)
	}
	// An operation on a closed socket raises OSError.
	if _, err := objects.CallMethod(cli, "recv", []objects.Object{objects.NewInt(1)}); err == nil {
		t.Errorf("recv after close did not raise")
	}
	call(conn, "close")
	call(srv, "close")
}

func TestSocketFamilyAttr(t *testing.T) {
	s := newSocket(t, 2, 1)
	defer func() { _, _ = objects.CallMethod(s, "close", nil) }()
	fam, err := objects.LoadAttr(s, "family")
	if err != nil {
		t.Fatalf("family: %v", err)
	}
	if n, _ := objects.AsInt(fam); n != 2 {
		t.Errorf("family = %v, want 2", fam)
	}
	// family is read-only: assigning it raises.
	if err := objects.StoreAttr(s, "family", objects.NewInt(9)); err == nil {
		t.Errorf("assigning family did not raise")
	}
}
