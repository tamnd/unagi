package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func socketFn(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("_socket")
	if err != nil {
		t.Fatalf("import _socket: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("_socket.%s: %v", name, err)
	}
	return fn
}

func TestSocketByteOrder(t *testing.T) {
	// htons and ntohs invert each other; on a little-endian build htons(1) swaps
	// to 256, and the round trip returns 1 whatever the host endianness.
	hs := socketFn(t, "htons")
	ns := socketFn(t, "ntohs")
	v, err := objects.Call(hs, []objects.Object{objects.NewInt(1)})
	if err != nil {
		t.Fatalf("htons: %v", err)
	}
	back, err := objects.Call(ns, []objects.Object{v})
	if err != nil {
		t.Fatalf("ntohs: %v", err)
	}
	if n, _ := objects.AsInt(back); n != 1 {
		t.Errorf("ntohs(htons(1)) = %v, want 1", back)
	}
	// A negative value is a ValueError; an out-of-width value is an OverflowError.
	if _, err := objects.Call(hs, []objects.Object{objects.NewInt(-1)}); err == nil {
		t.Errorf("htons(-1) did not raise")
	}
	if _, err := objects.Call(hs, []objects.Object{objects.NewInt(70000)}); err == nil {
		t.Errorf("htons(70000) did not raise")
	}
}

func TestSocketAddressRoundTrip(t *testing.T) {
	aton := socketFn(t, "inet_aton")
	ntoa := socketFn(t, "inet_ntoa")
	packed, err := objects.Call(aton, []objects.Object{objects.NewStr("1.2.3.4")})
	if err != nil {
		t.Fatalf("inet_aton: %v", err)
	}
	if b, _ := objects.AsBytes(packed); len(b) != 4 || b[0] != 1 || b[3] != 4 {
		t.Errorf("inet_aton('1.2.3.4') = %v", packed)
	}
	dotted, err := objects.Call(ntoa, []objects.Object{packed})
	if err != nil {
		t.Fatalf("inet_ntoa: %v", err)
	}
	if s, _ := objects.AsStr(dotted); s != "1.2.3.4" {
		t.Errorf("inet_ntoa round trip = %q", s)
	}
	// A malformed address is an error, not a silent result.
	if _, err := objects.Call(aton, []objects.Object{objects.NewStr("1.2.3.4.5")}); err == nil {
		t.Errorf("inet_aton('1.2.3.4.5') did not raise")
	}
}

func TestSocketErrorsAndConstants(t *testing.T) {
	mo, err := ImportModule("_socket")
	if err != nil {
		t.Fatalf("import _socket: %v", err)
	}
	// error is the OSError class itself; gaierror and herror are subclasses.
	errCls, _ := objects.LoadAttr(mo, "error")
	osCls, _ := objects.ExcClassValue("OSError")
	if errCls != osCls {
		t.Errorf("_socket.error is not OSError")
	}
	// AF_INET is the IANA value on every platform.
	af, _ := objects.LoadAttr(mo, "AF_INET")
	if n, _ := objects.AsInt(af); n != 2 {
		t.Errorf("AF_INET = %v, want 2", af)
	}
}
