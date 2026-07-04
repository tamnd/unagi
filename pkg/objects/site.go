package objects

import "fmt"

// The site builtins: exit, quit, copyright, credits, license, and help.
// CPython builds these in the site module, not as C builtins: exit and quit
// are Quitter instances, copyright/credits/license are _sitebuiltins._Printer
// instances, and help is a _sitebuiltins._Helper. unagi keeps the same value
// identity, repr, and type name so repr(exit), type(copyright).__name__, and
// calling them behave like the oracle. copyright and credits reproduce the
// oracle text exactly. license and help are honest stubs: their call writes
// the same short line their repr shows rather than paging the full license
// file or opening an interactive session, since neither the io stack nor an
// interactive reader exists here. That divergence is intentional and stays
// documented until those pieces land.

// quitterObject backs exit and quit. Calling it raises SystemExit(code) with
// code defaulting to None, so exit() terminates with status 0, exit(3) with 3,
// and exit("bye") prints the message and exits 1, all through the SystemExit
// handling in the generated main.
type quitterObject struct{ name string }

func (*quitterObject) TypeName() string { return "Quitter" }

// NewQuitter builds the exit or quit singleton with the given name, which its
// repr echoes.
func NewQuitter(name string) Object { return &quitterObject{name: name} }

// call raises SystemExit for exit()/quit(). Zero or one argument is the exit
// code (None when omitted); more arguments give CPython's arity error, whose
// count includes the bound self, so exit(1, 2) reports three.
func (q *quitterObject) call(args []Object) (Object, error) {
	if len(args) > 1 {
		return nil, Raise(TypeError,
			"Quitter.__call__() takes from 1 to 2 positional arguments but %d were given",
			len(args)+1)
	}
	code := None
	if len(args) == 1 {
		code = args[0]
	}
	return nil, NewException("SystemExit", []Object{code})
}

// printerObject backs copyright, credits, license, and help. reprText is the
// str/repr output; callText is written to stdout when the object is called.
// typeName is "_Printer" for the copyright trio and "_Helper" for help, so
// type(copyright).__name__ and type(help).__name__ report what the oracle does.
type printerObject struct {
	name     string
	typeName string
	reprText string
	callText string
}

func (p *printerObject) TypeName() string { return p.typeName }

// NewPrinter builds a copyright/credits/license singleton.
func NewPrinter(name, reprText, callText string) Object {
	return &printerObject{name: name, typeName: "_Printer", reprText: reprText, callText: callText}
}

// NewHelper builds the help singleton.
func NewHelper(name, reprText, callText string) Object {
	return &printerObject{name: name, typeName: "_Helper", reprText: reprText, callText: callText}
}

// siteWrite routes a printer's output through the same sink print uses so a
// swapped stdout carries; runtime wires it at init. A call before wiring, or
// in a context with no runtime, writes nothing.
var siteWrite func(string)

// SetSiteWrite installs the sink _Printer/_Helper output goes to.
func SetSiteWrite(w func(string)) { siteWrite = w }

// call writes the printer's text and returns None. A _Printer takes no
// arguments; more give CPython's arity error. A _Helper accepts and ignores
// arguments here, since the object introspection help(x) would show is a
// later slice.
func (p *printerObject) call(args []Object) (Object, error) {
	if p.typeName == "_Printer" && len(args) > 0 {
		return nil, Raise(TypeError,
			"_Printer.__call__() takes 1 positional argument but %d were given",
			len(args)+1)
	}
	if siteWrite != nil {
		siteWrite(p.callText)
	}
	return None, nil
}

// SystemExitCode maps an uncaught SystemExit to a process exit status the way
// CPython's runtime does: no code or a None code exits 0, an integer (or bool)
// code exits with that value, and any other code has its str written to stderr
// before exiting 1. The message writer is passed in so the runtime routes it
// through the same stderr sink the traceback printer uses. ok is false when the
// exception is not a SystemExit, so the caller falls back to a traceback.
func SystemExitCode(e *Exception, writeErr func(string)) (int, bool) {
	if e.Kind != "SystemExit" {
		return 0, false
	}
	var code Object
	switch len(e.Args) {
	case 0:
		return 0, true
	case 1:
		code = e.Args[0]
	default:
		code = NewTuple(append([]Object{}, e.Args...))
	}
	if code == None {
		return 0, true
	}
	if n, isInt := AsInt(code); isInt {
		return int(n), true
	}
	if writeErr != nil {
		writeErr(fmt.Sprintf("%s\n", Str(code)))
	}
	return 1, true
}
