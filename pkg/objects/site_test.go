package objects

import "testing"

func TestQuitterReprAndCall(t *testing.T) {
	exit := NewQuitter("exit")
	if got := Repr(exit); got != "Use exit() or Ctrl-D (i.e. EOF) to exit" {
		t.Fatalf("repr(exit) = %q", got)
	}
	if exit.TypeName() != "Quitter" {
		t.Fatalf("type name = %q, want Quitter", exit.TypeName())
	}

	// exit() raises SystemExit(None): args carry the single None and code reads
	// None too.
	_, err := Call(exit, nil)
	e, ok := err.(*Exception)
	if !ok || e.Kind != "SystemExit" {
		t.Fatalf("exit() error = %v, want SystemExit", err)
	}
	if len(e.Args) != 1 || e.Args[0] != None {
		t.Fatalf("exit() args = %v, want (None,)", e.Args)
	}
	code, cerr := excLoadAttr(e, "code")
	if cerr != nil || code != None {
		t.Fatalf("exit().code = %v, %v, want None", code, cerr)
	}

	// exit(7) carries the given code.
	_, err = Call(exit, []Object{NewInt(7)})
	e = err.(*Exception)
	if code, _ = excLoadAttr(e, "code"); Repr(code) != "7" {
		t.Fatalf("exit(7).code = %v, want 7", code)
	}

	// Too many arguments give CPython's arity error, counting the bound self.
	_, err = Call(exit, []Object{NewInt(1), NewInt(2)})
	if !isKind(err, TypeError) {
		t.Fatalf("exit(1, 2) error = %v, want TypeError", err)
	}
	if e := err.(*Exception); e.Text() != "Quitter.__call__() takes from 1 to 2 positional arguments but 3 were given" {
		t.Fatalf("exit(1, 2) text = %q", e.Text())
	}
}

func TestPrinterReprAndCall(t *testing.T) {
	var buf string
	SetSiteWrite(func(s string) { buf += s })
	t.Cleanup(func() { SetSiteWrite(nil) })

	cr := NewPrinter("copyright", "REPR", "CALL")
	if cr.TypeName() != "_Printer" {
		t.Fatalf("type name = %q, want _Printer", cr.TypeName())
	}
	if got := Repr(cr); got != "REPR" {
		t.Fatalf("repr = %q, want REPR", got)
	}
	if _, err := Call(cr, nil); err != nil {
		t.Fatalf("copyright() error: %v", err)
	}
	if buf != "CALL" {
		t.Fatalf("call wrote %q, want CALL", buf)
	}

	// A _Printer takes no arguments.
	if _, err := Call(cr, []Object{NewInt(1)}); !isKind(err, TypeError) {
		t.Fatalf("copyright(1) error = %v, want TypeError", err)
	}

	// A _Helper reports its own type and accepts (ignores) arguments.
	hp := NewHelper("help", "H", "H")
	if hp.TypeName() != "_Helper" {
		t.Fatalf("help type name = %q, want _Helper", hp.TypeName())
	}
	buf = ""
	if _, err := Call(hp, []Object{NewInt(1)}); err != nil {
		t.Fatalf("help(1) error: %v", err)
	}
	if buf != "H" {
		t.Fatalf("help(1) wrote %q, want H", buf)
	}
}

func TestSystemExitCode(t *testing.T) {
	for _, tc := range []struct {
		args []Object
		code int
		msg  string
	}{
		{nil, 0, ""},
		{[]Object{None}, 0, ""},
		{[]Object{NewInt(5)}, 5, ""},
		{[]Object{NewBool(true)}, 1, ""},
		{[]Object{NewStr("bye")}, 1, "bye\n"},
		{[]Object{NewInt(1), NewInt(2)}, 1, "(1, 2)\n"},
	} {
		var msg string
		e := NewException("SystemExit", tc.args)
		code, ok := SystemExitCode(e, func(s string) { msg += s })
		if !ok {
			t.Fatalf("args %v: ok=false", tc.args)
		}
		if code != tc.code {
			t.Fatalf("args %v: code=%d, want %d", tc.args, code, tc.code)
		}
		if msg != tc.msg {
			t.Fatalf("args %v: msg=%q, want %q", tc.args, msg, tc.msg)
		}
	}

	// A non-SystemExit exception is not our concern.
	if _, ok := SystemExitCode(NewException("ValueError", nil), nil); ok {
		t.Fatal("ValueError reported as a SystemExit")
	}
}

func TestSystemExitCodeAttrOnlySystemExit(t *testing.T) {
	e := NewException("ValueError", []Object{NewStr("x")})
	if _, err := excLoadAttr(e, "code"); !isKind(err, AttributeError) {
		t.Fatalf("ValueError.code error = %v, want AttributeError", err)
	}
}
