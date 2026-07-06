// Package lower turns the frontend AST into a Go main package. The boxed
// tier is the only tier until M4: every value is an objects.Object, every
// fallible operation runs through pkg/objects, and expression trees flatten
// into checked temporaries. The output is deterministic and gofmt-clean;
// making it pretty is the typed tier's job in later milestones.
//
// Every generated statement, expression, and declaration is built as a go/ast
// node and composed by nesting nodes, so a malformed splice is a type error
// at construction rather than invalid Go discovered at format time. The only
// place nodes become text is the assembly seam in Module; the node
// constructors live in goast.go.
//
// The lowering itself is spread by concern: funcgen.go builds function
// bodies, stmt.go and expr.go handle the plain statement and expression
// forms, call.go resolves callees, and exceptions.go owns try, raise, and
// assert.
package lower

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/unagi/pkg/frontend"
)

// Error is a compile-time rejection with a source position, for constructs
// the lowering does not handle yet.
type Error struct {
	File string
	Pos  frontend.Pos
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: error: %s", e.File, e.Pos.Line, e.Pos.Col, e.Msg)
}

// Module lowers a parsed module to a complete Go main package. file is the
// source path as given on the command line; it lands in the generated
// header, in compile errors, and in traceback frames. source is the raw
// Python text, embedded in the binary so tracebacks can print excerpt
// lines; nil skips the embedding and frames render bare, which is what
// CPython prints when the source file is gone.
func Module(mod *frontend.Module, file string, source []byte) ([]byte, error) {
	return lowerModule(mod, file, source, "__main__", false)
}

// PyModule lowers an imported module to a Go package the build lays out under
// pym/<name>. name is the import name: it becomes the module's __name__, the
// leading segment of every class qualname defined in it, and the pym_<name>
// package name. Instead of func main the package exposes Exec, which binds
// the module object's live slots and runs the body once; the generated
// modtable.go registers it in the runtime's import table.
func PyModule(mod *frontend.Module, name, file string, source []byte) ([]byte, error) {
	return lowerModule(mod, file, source, name, true)
}

func lowerModule(mod *frontend.Module, file string, source []byte, modName string, pkgMode bool) ([]byte, error) {
	// Rewrite class-private __names to their mangled _Class__name form before
	// any name analysis runs, so scope collection and lowering see the same
	// identifiers CPython would after mangling.
	frontend.MangleClassPrivates(mod)

	e := &emitter{file: file, source: source, modName: modName, pkgMode: pkgMode, escWarns: mod.EscapeWarnings, defs: map[string]*frontend.FuncDef{}, defOrd: map[string]int{}, rebound: map[string]bool{}, globalDecl: map[string]bool{}, moduleVars: map[string]bool{}, classOrd: map[string]int{}}

	var body []frontend.Stmt
	var defs []*frontend.FuncDef
	var classes []*frontend.ClassDef
	// Classes are collected through nested module-level blocks too (a class
	// statement under try or if is common for creation-error probing and
	// conditional definition), so their methods emit and their ordinals stay
	// unique; def bodies are excluded since a class in a function is rejected
	// at lowering.
	var collectClasses func(list []frontend.Stmt) error
	collectClasses = func(list []frontend.Stmt) error {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.ClassDef:
				if _, dup := e.classOrd[s.Name]; dup {
					return e.errf(s.Span(), "redefining class %q is not supported yet", s.Name)
				}
				e.classOrd[s.Name] = len(classes)
				classes = append(classes, s)
			case *frontend.If:
				if err := collectClasses(s.Body); err != nil {
					return err
				}
				if err := collectClasses(s.Else); err != nil {
					return err
				}
			case *frontend.While:
				if err := collectClasses(s.Body); err != nil {
					return err
				}
				if err := collectClasses(s.Else); err != nil {
					return err
				}
			case *frontend.For:
				if err := collectClasses(s.Body); err != nil {
					return err
				}
				if err := collectClasses(s.Else); err != nil {
					return err
				}
			case *frontend.With:
				if err := collectClasses(s.Body); err != nil {
					return err
				}
			case *frontend.Try:
				if err := collectClasses(s.Body); err != nil {
					return err
				}
				for _, h := range s.Handlers {
					if err := collectClasses(h.Body); err != nil {
						return err
					}
				}
				if err := collectClasses(s.OrElse); err != nil {
					return err
				}
				if err := collectClasses(s.Final); err != nil {
					return err
				}
			case *frontend.Match:
				for _, c := range s.Cases {
					if err := collectClasses(c.Body); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := collectClasses(mod.Body); err != nil {
		return nil, err
	}
	for _, s := range mod.Body {
		if d, ok := s.(*frontend.FuncDef); ok {
			if _, dup := e.defs[d.Name]; dup {
				// Redefinition is legal Python; the lowering hoists defs, so
				// the second binding cannot take effect and we refuse it.
				return nil, e.errf(d.Span(), "redefining function %q is not supported yet", d.Name)
			}
			e.defOrd[d.Name] = len(defs)
			for _, p := range d.Params {
				if p.Default != nil {
					e.slots = append(e.slots, e.slotName(d.Name, p.Name))
					e.usedObjects = true
				}
			}
			e.defs[d.Name] = d
			defs = append(defs, d)
		}
		// Defs stay in the body: the def statement evaluates its parameter
		// defaults when it executes, so the slot fills at that point.
		body = append(body, s)
	}

	// A def name assigned or deleted anywhere at module scope, or declared
	// global by some def, becomes an ordinary checked module variable: the
	// def statement binds it to the function object, later statements can
	// rebind it, and every read and call goes through the variable instead
	// of the static fast path.
	assigned := map[string]bool{}
	collectAssigned(body, assigned)
	for _, d := range defs {
		collectGlobals(d.Body, e.globalDecl)
	}
	for n := range assigned {
		e.moduleVars[n] = true
	}
	for n := range e.globalDecl {
		e.moduleVars[n] = true
	}
	// A decorated def binds its name to whatever the decorators return, an
	// arbitrary object, so the name becomes a checked module variable read and
	// called dynamically rather than through the static function fast path.
	for _, d := range defs {
		if len(d.Decorators) > 0 {
			e.moduleVars[d.Name] = true
		}
	}
	// In a module package every def is a module variable: an importer can
	// rebind or delete m.f, and calls inside the module must observe that, so
	// no name keeps the static fast path.
	if pkgMode {
		for n := range e.defs {
			e.moduleVars[n] = true
		}
	}
	for n := range e.moduleVars {
		if _, ok := e.defs[n]; ok {
			e.rebound[n] = true
		}
	}
	if len(defs) > 0 || len(e.moduleVars) > 0 || len(classes) > 0 {
		e.usedObjects = true
	}

	var fnDecls []*ast.FuncDecl
	for _, d := range defs {
		decl, err := e.emitFunc(d)
		if err != nil {
			return nil, err
		}
		fnDecls = append(fnDecls, decl)
	}
	var methodDecls []methodEmit
	for _, c := range classes {
		ms, err := e.emitClassMethods(c)
		if err != nil {
			return nil, err
		}
		methodDecls = append(methodDecls, ms...)
	}
	pymain, err := e.emitMain(body)
	if err != nil {
		return nil, err
	}
	e.prependWarnings(pymain)

	// The assembly seam: the one place nodes become text. The header comment,
	// package clause, and import block are plain lines, each declaration
	// prints through format.Node, and each def's doc comment goes down as a
	// text line right before its decl. Nothing above this point writes Go
	// syntax as strings.
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by unagi from %s. DO NOT EDIT.\n", file)
	if pkgMode {
		// A module package always references objects (the Exec signature) and
		// runtime (the source registration), and never os: there is no main.
		fmt.Fprintf(&out, "package pym_%s\n\nimport (\n", modName)
		out.WriteString("\t\"github.com/tamnd/unagi/pkg/objects\"\n")
		out.WriteString("\t\"github.com/tamnd/unagi/pkg/runtime\"\n)\n\n")
	} else {
		out.WriteString("package main\n\nimport (\n\t\"os\"\n\n")
		if e.usedObjects {
			out.WriteString("\t\"github.com/tamnd/unagi/pkg/objects\"\n")
		}
		out.WriteString("\t\"github.com/tamnd/unagi/pkg/runtime\"\n)\n\n")
	}
	if e.usedTB || pkgMode {
		fmt.Fprintf(&out, "// pyFile is the source path traceback frames cite.\nconst pyFile = %s\n\n", strconv.Quote(file))
		// A module package registers unconditionally: the init call is also
		// what keeps the runtime import used when the body needs nothing else.
		if len(source) > 0 || pkgMode {
			fmt.Fprintf(&out, "// pySource is the embedded source, so tracebacks can quote the line\n// under each frame the way CPython does.\nconst pySource = %s\n\nfunc init() { runtime.RegisterSource(pyFile, pySource) }\n\n", strconv.Quote(string(source)))
		}
	}
	if pkgMode {
		out.WriteString("// thisModule is the module object, set by Exec before the body runs.\n// Reads of names this compile never saw route through it, so an attribute\n// an importer sets on the module is visible inside it.\nvar thisModule *objects.Module\n\n")
	}
	if len(e.slots) > 0 {
		out.WriteString("// Parameter default slots, assigned when each def statement runs.\nvar (\n")
		for _, s := range e.slots {
			fmt.Fprintf(&out, "\t%s objects.Object\n", s)
		}
		out.WriteString(")\n\n")
	}
	if len(defs) > 0 {
		out.WriteString("// Function objects, built when each def statement runs.\nvar (\n")
		for _, d := range defs {
			fmt.Fprintf(&out, "\t%s objects.Object\n", e.fnObjName(d.Name))
		}
		out.WriteString(")\n\n")
	}
	if len(e.moduleVars) > 0 {
		out.WriteString("// Module-scope variables, at package level so def bodies reach them.\nvar (\n")
		for _, n := range sortedNames(e.moduleVars) {
			fmt.Fprintf(&out, "\t%s objects.Object\n", mangle(n))
		}
		out.WriteString(")\n\n")
	}
	if pkgMode {
		out.WriteString("// Exec binds every module-scope variable as a live slot on the module\n// object, then runs the body. The import machinery calls it at most once.\n")
		if err := writeDecl(&out, execDecl(sortedNames(e.moduleVars))); err != nil {
			return nil, err
		}
	} else {
		if err := writeDecl(&out, mainDecl()); err != nil {
			return nil, err
		}
	}
	if err := writeDecl(&out, pymain); err != nil {
		return nil, err
	}
	for i, decl := range fnDecls {
		fmt.Fprintf(&out, "// %s is Python def %s.\n", e.defName(defs[i].Name), defs[i].Name)
		if err := writeDecl(&out, decl); err != nil {
			return nil, err
		}
		// The adapter lives at package level so it sees the def's Go
		// function even when a rebound def name shadows it in pymain.
		fmt.Fprintf(&out, "// %s adapts %s to the function object calling convention.\n",
			e.implName(defs[i].Name), e.defName(defs[i].Name))
		if err := writeDecl(&out, e.implDecl(defs[i])); err != nil {
			return nil, err
		}
	}
	for _, m := range methodDecls {
		fmt.Fprintf(&out, "// %s\n", m.doc)
		if err := writeDecl(&out, m.decl); err != nil {
			return nil, err
		}
		fmt.Fprintf(&out, "// %s\n", m.implDoc)
		if err := writeDecl(&out, m.impl); err != nil {
			return nil, err
		}
	}

	formatted, ferr := format.Source(out.Bytes())
	if ferr != nil {
		// A formatting failure is an emitter bug; surface the raw source so
		// it can be diagnosed from the error alone.
		return nil, fmt.Errorf("lower: generated invalid Go (%v):\n%s", ferr, out.String())
	}
	return formatted, nil
}

type emitter struct {
	file        string
	source      []byte
	modName     string // the module's __name__: "__main__" or the import name
	pkgMode     bool   // emitting an importable module package, not package main
	defs        map[string]*frontend.FuncDef
	defOrd      map[string]int
	rebound     map[string]bool // def names that are also module variables
	globalDecl  map[string]bool // names some def declares global
	moduleVars  map[string]bool // every module-scope variable, emitted at package level
	slots       []string
	classOrd    map[string]int           // top-level class name to its emission ordinal
	finWarns    []finWarn                // PEP 765 finally-jump SyntaxWarnings, in discovery order
	escWarns    []frontend.EscapeWarning // invalid backslash-escape SyntaxWarnings from the lexer
	usedObjects bool
	usedTB      bool
}

// finWarn records one return/break/continue that exits a finally block. CPython
// 3.14 prints a PEP 765 SyntaxWarning for each at compile time; the generated
// program replays them to stderr before the module body runs.
type finWarn struct {
	line int
	kind string // "return", "break", or "continue"
}

// warnFinallyJump records a finally-exiting jump for the PEP 765 warning, once
// per source line so a line visited twice cannot double-print.
func (e *emitter) warnFinallyJump(line int, kind string) {
	for _, w := range e.finWarns {
		if w.line == line {
			return
		}
	}
	e.finWarns = append(e.finWarns, finWarn{line: line, kind: kind})
}

// sourceLine returns the 1-indexed source line with leading whitespace removed,
// the text CPython quotes under a SyntaxWarning. An out-of-range line gives "".
func (e *emitter) sourceLine(n int) string {
	if n < 1 {
		return ""
	}
	lines := strings.Split(string(e.source), "\n")
	if n > len(lines) {
		return ""
	}
	return strings.TrimLeft(lines[n-1], " \t")
}

// compileWarn is one compile-time SyntaxWarning ready to replay: its source
// position orders it against the others, and msg is the fully formatted text.
type compileWarn struct {
	line, col int
	msg       string
}

// prependWarnings injects one runtime.SyntaxWarn call per recorded compile-time
// warning at the top of pymain, sorted by source position so the program
// replays them in the order CPython's compiler emitted them. Both the PEP 765
// finally-jump warnings and the invalid backslash-escape warnings flow through
// here. Each call carries the fully formatted message, so the runtime helper
// only writes it to stderr.
func (e *emitter) prependWarnings(pymain *ast.FuncDecl) {
	var warns []compileWarn
	for _, w := range e.finWarns {
		warns = append(warns, compileWarn{
			line: w.line,
			msg: fmt.Sprintf("%s:%d: SyntaxWarning: '%s' in a 'finally' block\n  %s\n",
				e.file, w.line, w.kind, e.sourceLine(w.line)),
		})
	}
	for _, w := range e.escWarns {
		warns = append(warns, compileWarn{
			line: w.Line,
			col:  w.Col,
			msg: fmt.Sprintf("%s:%d: SyntaxWarning: \"\\%s\" is an invalid escape sequence. Such sequences will not work in the future. Did you mean \"\\\\%s\"? A raw string is also an option.\n  %s\n",
				e.file, w.Line, w.Char, w.Char, e.sourceLine(w.Line)),
		})
	}
	if len(warns) == 0 {
		return
	}
	sort.SliceStable(warns, func(i, j int) bool {
		if warns[i].line != warns[j].line {
			return warns[i].line < warns[j].line
		}
		return warns[i].col < warns[j].col
	})
	var stmts []ast.Stmt
	for _, w := range warns {
		stmts = append(stmts, exprStmt(callExpr(sel("runtime", "SyntaxWarn"), strLit(w.msg))))
	}
	pymain.Body.List = append(stmts, pymain.Body.List...)
}

// methodDefName is the Go function carrying one method's body. The class and
// method ordinals keep it unique across classes and away from the def and
// module-variable namespaces.
func (e *emitter) methodDefName(className, methodName string, mi int) string {
	return fmt.Sprintf("clsdef%d_%d_%s", e.classOrd[className], mi, methodName)
}

// methodImplName is the adapter that turns one method's Go function into the
// slice-taking implementation its function object carries.
func (e *emitter) methodImplName(className, methodName string, mi int) string {
	return fmt.Sprintf("clsimpl%d_%d_%s", e.classOrd[className], mi, methodName)
}

// slotName is the module-level variable holding one parameter default,
// evaluated when the def statement runs. The def ordinal keeps names unique
// without leaning on the mangled namespace.
func (e *emitter) slotName(fname, pname string) string {
	return fmt.Sprintf("dflt%d_%s", e.defOrd[fname], pname)
}

// fnObjName is the module-level variable holding one def's function object,
// built when the def statement runs.
func (e *emitter) fnObjName(fname string) string {
	return fmt.Sprintf("fn%d_%s", e.defOrd[fname], fname)
}

// implName is the package-level adapter turning one def's Go function into
// the slice-taking implementation a function object carries.
func (e *emitter) implName(fname string) string {
	return fmt.Sprintf("impl%d_%s", e.defOrd[fname], fname)
}

// defName is the Go function that carries one def's body. It has its own
// namespace so a rebound def name, which also becomes a mangled module
// variable, does not collide with the function implementing it.
func (e *emitter) defName(fname string) string {
	return fmt.Sprintf("def%d_%s", e.defOrd[fname], fname)
}

// implDecl builds one def's adapter: unpack the bound argument slice into
// the def's Go parameters.
func (e *emitter) implDecl(d *frontend.FuncDef) *ast.FuncDecl {
	return e.implDeclAs(d, e.implName(d.Name), e.defName(d.Name))
}

// implDeclAs builds the adapter that turns a Go function taking positional
// parameters into the slice-taking implementation a function object carries.
// implName is the adapter's Go name and target is the Go function it calls.
func (e *emitter) implDeclAs(d *frontend.FuncDef, implName, target string) *ast.FuncDecl {
	args := make([]ast.Expr, len(d.Params))
	for i := range d.Params {
		args[i] = &ast.IndexExpr{X: ident("args"), Index: intLit(strconv.Itoa(i))}
	}
	return &ast.FuncDecl{
		Name: ident(implName),
		Type: e.implType(),
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{
			Results: []ast.Expr{callExpr(ident(target), args...)},
		}}},
	}
}

func (e *emitter) errf(pos frontend.Pos, format string, args ...any) error {
	return &Error{File: e.file, Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

// obj marks pkg/objects as imported and returns the qualified name node, so
// the import list stays exact even for programs that never touch a value.
func (e *emitter) obj(name string) ast.Expr {
	e.usedObjects = true
	return sel("objects", name)
}

func mangle(name string) string { return "u_" + name }
