package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// hashCtor imports a builtin hash module and returns its named constructor.
func hashCtor(t *testing.T, module, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule(module)
	if err != nil {
		t.Fatalf("import %s: %v", module, err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("%s.%s: %v", module, name, err)
	}
	return fn
}

// hexdigest calls a hash object's hexdigest() and returns the string.
func hexdigest(t *testing.T, h objects.Object, args ...objects.Object) string {
	t.Helper()
	r, err := objects.CallMethod(h, "hexdigest", args)
	if err != nil {
		t.Fatalf("hexdigest: %v", err)
	}
	s, ok := objects.AsStr(r)
	if !ok {
		t.Fatalf("hexdigest returned %s, want str", r.TypeName())
	}
	return s
}

// TestHashBuiltinDigests checks each builtin module's constructor produces the
// CPython 3.14.6 digest for a fixed message, and reports the right digest_size
// and block_size. The expected hex is from _md5/_sha1/_sha2/_sha3 directly.
func TestHashBuiltinDigests(t *testing.T) {
	msg := objects.NewBytes([]byte("The quick brown fox"))
	cases := []struct {
		module, name, hex string
		digestSize, block int64
	}{
		{"_md5", "md5", "a2004f37730b9445670a738fa0fc9ee5", 16, 64},
		{"_sha1", "sha1", "c519c1a06cdbeb2bc499e22137fb48683858b345", 20, 64},
		{"_sha2", "sha224", "146129bd31db775c819f4c28929cc5ac7130d4bac470e97134857c1a", 28, 64},
		{"_sha2", "sha256", "5cac4f980fedc3d3f1f99b4be3472c9b30d56523e632d151237ec9309048bda9", 32, 64},
		{"_sha2", "sha384", "2e45933dd1a1e6a6928a732d58abeb180c225e5e7b99c64eb6f233a7b99ee4635c17f44ca544cf620cf4289deb4c08cf", 48, 128},
		{"_sha2", "sha512", "015e6d23e760f612cca616c54f110cb12dd54213f1e046c7607081372402eff4936b379296ed549236020afb37bd3e728a044a4243754f095498c98bc24f77e0", 64, 128},
		{"_sha3", "sha3_224", "c0144010bf2c56c005a2cb6b26cc7d2cce38a9ce8038babf0867cf6d", 28, 144},
		{"_sha3", "sha3_256", "6a982a1fd2b2b33bf284768bd0e3df2a67ee3d412ae6f3df8248b268755751d4", 32, 136},
		{"_sha3", "sha3_384", "c82f7d2460b9dc489e9dea134c694de251b64e4b6862f7b8742fbce9d121e8e9b8bbc44972c1543f2fc885a398c191bc", 48, 104},
		{"_sha3", "sha3_512", "ca5ce3b1097b2cc9b6d63e234b9db237537b0c2bbedb746ca6df1ef7d06ec50b72745167ba4f171b77367bd66272d5d0d8af02e5df14bf0f2dbddfcb11a788e3", 64, 72},
	}
	for _, c := range cases {
		ctor := hashCtor(t, c.module, c.name)
		h, err := objects.Call(ctor, []objects.Object{msg})
		if err != nil {
			t.Fatalf("%s(): %v", c.name, err)
		}
		if got := hexdigest(t, h); got != c.hex {
			t.Errorf("%s hexdigest = %s, want %s", c.name, got, c.hex)
		}
		ds, err := objects.LoadAttr(h, "digest_size")
		if err != nil {
			t.Fatalf("%s.digest_size: %v", c.name, err)
		}
		if n, _ := objects.AsInt(ds); n != c.digestSize {
			t.Errorf("%s digest_size = %d, want %d", c.name, n, c.digestSize)
		}
		bs, err := objects.LoadAttr(h, "block_size")
		if err != nil {
			t.Fatalf("%s.block_size: %v", c.name, err)
		}
		if n, _ := objects.AsInt(bs); n != c.block {
			t.Errorf("%s block_size = %d, want %d", c.name, n, c.block)
		}
		nm, err := objects.LoadAttr(h, "name")
		if err != nil {
			t.Fatalf("%s.name: %v", c.name, err)
		}
		if s, _ := objects.AsStr(nm); s != c.name {
			t.Errorf("%s name = %q, want %q", c.name, s, c.name)
		}
	}
}

// TestHashBuiltinShake checks the two shake XOFs from _sha3 produce their
// variable-length CPython digests via hexdigest(length).
func TestHashBuiltinShake(t *testing.T) {
	msg := objects.NewBytes([]byte("The quick brown fox"))
	cases := []struct{ name, hex16 string }{
		{"shake_128", "234cdabf1973d6a5db2100395d798a26"},
		{"shake_256", "1dd0a51c9c3f25399947ace094f380a6"},
	}
	for _, c := range cases {
		ctor := hashCtor(t, "_sha3", c.name)
		h, err := objects.Call(ctor, []objects.Object{msg})
		if err != nil {
			t.Fatalf("%s(): %v", c.name, err)
		}
		if got := hexdigest(t, h, objects.NewInt(16)); got != c.hex16 {
			t.Errorf("%s hexdigest(16) = %s, want %s", c.name, got, c.hex16)
		}
	}
}

// TestHashBuiltinIncremental checks update() accumulates and copy() forks the
// state independently, the way a chunked read into sha256 must.
func TestHashBuiltinIncremental(t *testing.T) {
	ctor := hashCtor(t, "_sha2", "sha256")
	h, err := objects.Call(ctor, nil)
	if err != nil {
		t.Fatalf("sha256(): %v", err)
	}
	if _, err := objects.CallMethod(h, "update", []objects.Object{objects.NewBytes([]byte("The quick "))}); err != nil {
		t.Fatalf("update: %v", err)
	}
	// copy() before the second chunk: the fork keeps the shorter message.
	fork, err := objects.CallMethod(h, "copy", nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if _, err := objects.CallMethod(h, "update", []objects.Object{objects.NewBytes([]byte("brown fox"))}); err != nil {
		t.Fatalf("update: %v", err)
	}
	const whole = "5cac4f980fedc3d3f1f99b4be3472c9b30d56523e632d151237ec9309048bda9"
	if got := hexdigest(t, h); got != whole {
		t.Errorf("incremental sha256 = %s, want %s", got, whole)
	}
	// The fork saw only "The quick "; it must not equal the whole-message digest.
	if got := hexdigest(t, fork); got == whole {
		t.Errorf("copy shared state with original: both %s", got)
	}
}
