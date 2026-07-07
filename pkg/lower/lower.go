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
	return lowerModule(mod, file, source, "__main__", false, nil)
}

// ModuleStars lowers the entry module of a program that uses star imports:
// stars maps each compiled module's import name to its export list, computed
// by ModuleExports, so `from m import *` can bind names statically.
func ModuleStars(mod *frontend.Module, file string, source []byte, stars map[string]StarExports) ([]byte, error) {
	return lowerModule(mod, file, source, "__main__", false, stars)
}

// PyModule lowers an imported module to a Go package the build lays out under
// pym/<name>. name is the import name: it becomes the module's __name__, the
// leading segment of every class qualname defined in it, and the pym_<name>
// package name. Instead of func main the package exposes Exec, which binds
// the module object's live slots and runs the body once; the generated
// modtable.go registers it in the runtime's import table.
func PyModule(mod *frontend.Module, name, file string, source []byte) ([]byte, error) {
	return lowerModule(mod, file, source, name, true, nil)
}

// PyModuleStars is PyModule for a program that uses star imports, threading
// the per-module export lists so a `from m import *` inside this module can
// bind names statically.
func PyModuleStars(mod *frontend.Module, name, file string, source []byte, stars map[string]StarExports) ([]byte, error) {
	return lowerModule(mod, file, source, name, true, stars)
}

// StarExports is one module's contribution to a `from m import *`. All holds a
// literal top-level __all__ list in source order when the module defines one;
// otherwise Names holds every module-scope public name, sorted. Exactly one is
// set: All drives the attribute-load form that can raise, Names the default
// rule that silently skips unbound names.
type StarExports struct {
	All   []string
	Names []string
}

// BuiltinStarExports is the star-import surface of the built-in modules the
// runtime provides in Go rather than compiling from source. A compiled module
// exposes its names through its parsed body, but a built-in module has no body
// the compiler can read, so `from _types import *` needs the export list
// spelled out here to bind the whole set. The list mirrors the names the
// matching runtime module registers; keep the two in step.
var BuiltinStarExports = map[string]StarExports{
	"_types": {All: []string{
		"AsyncGeneratorType", "BuiltinFunctionType", "BuiltinMethodType",
		"CapsuleType", "CellType", "ClassMethodDescriptorType", "CodeType",
		"CoroutineType", "EllipsisType", "FrameType", "FunctionType",
		"GeneratorType", "GenericAlias", "GetSetDescriptorType", "LambdaType",
		"MappingProxyType", "MemberDescriptorType", "MethodDescriptorType",
		"MethodType", "MethodWrapperType", "ModuleType", "NoneType",
		"NotImplementedType", "SimpleNamespace", "TracebackType", "UnionType",
		"WrapperDescriptorType",
	}},
}

// ModuleExports computes a module's star-import surface: a literal top-level
// __all__ of plain strings when present, else every module-scope bound name
// that does not start with an underscore. A non-literal __all__ is not modeled
// and falls back to the name rule, which is the closest static approximation.
func ModuleExports(mod *frontend.Module) StarExports {
	if all, ok := literalAll(mod.Body); ok {
		return StarExports{All: all}
	}
	bound := map[string]bool{}
	collectAssigned(mod.Body, bound)
	collectModuleDefs(mod.Body, bound)
	var names []string
	for n := range bound {
		if !strings.HasPrefix(n, "_") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return StarExports{Names: names}
}

// literalAll returns the value of a top-level `__all__ = [...]` or `= (...)`
// assignment when it is a literal list or tuple of string literals. The last
// such assignment wins, matching a straight-line module body. ok is false when
// no literal __all__ is present, including when __all__ is built dynamically.
func literalAll(body []frontend.Stmt) ([]string, bool) {
	var all []string
	found := false
	for _, s := range body {
		a, ok := s.(*frontend.Assign)
		if !ok || len(a.Targets) != 1 {
			continue
		}
		n, ok := a.Targets[0].(*frontend.Name)
		if !ok || n.Id != "__all__" {
			continue
		}
		var elts []frontend.Expr
		switch v := a.Value.(type) {
		case *frontend.ListLit:
			elts = v.Elts
		case *frontend.TupleLit:
			elts = v.Elts
		default:
			continue
		}
		names := make([]string, 0, len(elts))
		literal := true
		for _, el := range elts {
			sl, ok := el.(*frontend.StrLit)
			if !ok {
				literal = false
				break
			}
			names = append(names, sl.Val)
		}
		if literal {
			all = names
			found = true
		}
	}
	return all, found
}

// collectModuleDefs adds every module-scope def name to out, at any nesting of
// module-level blocks (a def under if or try still binds at module scope). It
// does not descend into def or class bodies, whose names are their own scope.
func collectModuleDefs(body []frontend.Stmt, out map[string]bool) {
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.FuncDef:
				out[s.Name] = true
			case *frontend.If:
				walk(s.Body)
				walk(s.Else)
			case *frontend.While:
				walk(s.Body)
				walk(s.Else)
			case *frontend.For:
				walk(s.Body)
				walk(s.Else)
			case *frontend.With:
				walk(s.Body)
			case *frontend.Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(body)
}

// RelativeName resolves a relative import against the importer's package:
// strip level-1 trailing segments of the package name, then append the
// written module path. ok is false when the dots walk past the top of the
// package tree. The build's module collector shares this arithmetic.
func RelativeName(pack string, level int, module string) (string, bool) {
	segs := strings.Split(pack, ".")
	remaining := len(segs) - (level - 1)
	if pack == "" || remaining < 1 {
		return "", false
	}
	base := strings.Join(segs[:remaining], ".")
	if module != "" {
		return base + "." + module, true
	}
	return base, true
}

func lowerModule(mod *frontend.Module, file string, source []byte, modName string, pkgMode bool, stars map[string]StarExports) ([]byte, error) {
	// Rewrite class-private __names to their mangled _Class__name form before
	// any name analysis runs, so scope collection and lowering see the same
	// identifiers CPython would after mangling.
	frontend.MangleClassPrivates(mod)

	e := &emitter{file: file, source: source, modName: modName, pkgMode: pkgMode, pkgInit: pkgMode && strings.HasSuffix(file, "__init__.py"), stars: stars, escWarns: mod.EscapeWarnings, defs: map[string]*frontend.FuncDef{}, defOrd: map[string]int{}, rebound: map[string]bool{}, globalDecl: map[string]bool{}, moduleVars: map[string]bool{}, classOrd: map[string]int{}}

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
	// Defs are collected through nested module-level blocks the same way
	// classes are, so a guarded fallback def (the try/except ImportError idiom
	// the stdlib floor leans on) registers as the canonical function and its
	// def statement takes effect where it runs. A def collected from inside a
	// block is conditional: its binding may not happen, so its name becomes a
	// checked module variable. Def bodies are not descended into; a nested
	// function is a separate scope handled at lowering.
	conditionalDefs := map[string]bool{}
	var collectDefs func(list []frontend.Stmt, conditional bool) error
	collectDefs = func(list []frontend.Stmt, conditional bool) error {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.FuncDef:
				if _, dup := e.defs[s.Name]; dup {
					// Redefinition is legal Python; the lowering hoists defs, so
					// the second binding cannot take effect and we refuse it.
					return e.errf(s.Span(), "redefining function %q is not supported yet", s.Name)
				}
				e.defOrd[s.Name] = len(defs)
				for _, p := range s.Params {
					if p.Default != nil {
						e.slots = append(e.slots, e.slotName(s.Name, p.Name))
						e.usedObjects = true
					}
				}
				e.defs[s.Name] = s
				defs = append(defs, s)
				if conditional {
					conditionalDefs[s.Name] = true
				}
			case *frontend.If:
				if err := collectDefs(s.Body, true); err != nil {
					return err
				}
				if err := collectDefs(s.Else, true); err != nil {
					return err
				}
			case *frontend.While:
				if err := collectDefs(s.Body, true); err != nil {
					return err
				}
				if err := collectDefs(s.Else, true); err != nil {
					return err
				}
			case *frontend.For:
				if err := collectDefs(s.Body, true); err != nil {
					return err
				}
				if err := collectDefs(s.Else, true); err != nil {
					return err
				}
			case *frontend.With:
				if err := collectDefs(s.Body, true); err != nil {
					return err
				}
			case *frontend.Try:
				if err := collectDefs(s.Body, true); err != nil {
					return err
				}
				for _, h := range s.Handlers {
					if err := collectDefs(h.Body, true); err != nil {
						return err
					}
				}
				if err := collectDefs(s.OrElse, true); err != nil {
					return err
				}
				if err := collectDefs(s.Final, true); err != nil {
					return err
				}
			case *frontend.Match:
				for _, c := range s.Cases {
					if err := collectDefs(c.Body, true); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := collectDefs(mod.Body, false); err != nil {
		return nil, err
	}
	// Defs stay in the body: the def statement evaluates its parameter defaults
	// when it executes, so the slot fills at that point.
	body = append(body, mod.Body...)

	// __doc__ is the module docstring: the value of a leading bare string
	// literal, otherwise None. CPython stores it before the rest of the body
	// runs and leaves it an ordinary rebindable variable, so a synthetic
	// __doc__ = <docstring or None> at the front of the body gives it exactly
	// that shape. The leading literal is consumed here rather than left to
	// emit a discarded expression statement.
	var docValue frontend.Expr = &frontend.NoneLit{}
	if len(body) > 0 {
		if es, ok := body[0].(*frontend.ExprStmt); ok {
			if sl, ok := es.X.(*frontend.StrLit); ok {
				docValue = sl
				body = body[1:]
			}
		}
	}
	body = append([]frontend.Stmt{&frontend.Assign{
		Targets: []frontend.Expr{&frontend.Name{Id: "__doc__"}},
		Value:   docValue,
	}}, body...)

	// A def name assigned or deleted anywhere at module scope, or declared
	// global by some def, becomes an ordinary checked module variable: the
	// def statement binds it to the function object, later statements can
	// rebind it, and every read and call goes through the variable instead
	// of the static fast path.
	assigned := map[string]bool{}
	collectAssigned(body, assigned)
	// A `from m import *` binds names the collector cannot see in the source,
	// so pull each resolved module's export list into the assigned set; those
	// names become checked module variables like any other module binding.
	e.collectStarNames(body, assigned)
	e.hasStar = hasModuleStar(body)
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
	// A conditionally defined name may never bind, so it is a checked module
	// variable too: the def statement binds it where it runs, and a read before
	// or without that raises NameError instead of resolving a function that was
	// never defined on this path.
	for n := range conditionalDefs {
		e.moduleVars[n] = true
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
		// A dotted module name folds its dots into underscores for the Go
		// package identifier; the directory layout keeps the real nesting.
		fmt.Fprintf(&out, "package pym_%s\n\nimport (\n", strings.ReplaceAll(modName, ".", "_"))
		out.WriteString("\t\"github.com/tamnd/unagi/pkg/objects\"\n")
		out.WriteString("\t\"github.com/tamnd/unagi/pkg/runtime\"\n)\n\n")
	} else {
		out.WriteString("package main\n\nimport (\n\t\"os\"\n\n")
		if e.usedObjects {
			out.WriteString("\t\"github.com/tamnd/unagi/pkg/objects\"\n")
		}
		out.WriteString("\t\"github.com/tamnd/unagi/pkg/runtime\"\n)\n\n")
	}
	if e.usedTB || pkgMode || e.usedGlobals {
		fmt.Fprintf(&out, "// pyFile is the source path traceback frames cite.\nconst pyFile = %s\n\n", strconv.Quote(file))
		// A module package registers unconditionally: the init call is also
		// what keeps the runtime import used when the body needs nothing else.
		if len(source) > 0 || pkgMode {
			fmt.Fprintf(&out, "// pySource is the embedded source, so tracebacks can quote the line\n// under each frame the way CPython does.\nconst pySource = %s\n\nfunc init() { runtime.RegisterSource(pyFile, pySource) }\n\n", strconv.Quote(string(source)))
		}
	}
	if pkgMode || e.usedGlobals {
		out.WriteString("// thisModule is the module object, bound before the body runs. Reads of\n// names this compile never saw route through it, so an attribute an importer\n// sets on the module is visible inside it, and globals() reads its namespace.\nvar thisModule *objects.Module\n\n")
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
	} else if e.usedGlobals {
		// A plain top-level def keeps the static fast path and is not a module
		// variable, so it is bound through its function-object slot; every other
		// module-scope name is a checked module variable already.
		var plainDefs []string
		for _, d := range defs {
			if !e.moduleVars[d.Name] {
				plainDefs = append(plainDefs, d.Name)
			}
		}
		out.WriteString("// main builds the __main__ module and binds every module-scope name onto\n// it before the body runs, so globals() reads the same live storage the body\n// writes, then runs the body.\n")
		if err := writeDecl(&out, e.mainGlobalsDecl(sortedNames(e.moduleVars), plainDefs)); err != nil {
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
	file    string
	source  []byte
	modName string                 // the module's __name__: "__main__" or the import name
	pkgMode bool                   // emitting an importable module package, not package main
	pkgInit bool                   // this module is a package's __init__.py
	stars   map[string]StarExports // export lists keyed by module name, for `from m import *`
	hasStar bool                   // a module-level `from m import *` makes unresolved names dynamic

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
	usedGlobals bool // the body calls globals(), so main binds a __main__ module
}

// selfPackage is the module's __package__ resolved at compile time: the
// module's own name for a package, the parent for a submodule, the empty
// string for a top-level module. known is false for the entry script, whose
// __package__ is None; both states make a relative import fail the same way.
func (e *emitter) selfPackage() (string, bool) {
	if !e.pkgMode {
		return "", false
	}
	if e.pkgInit {
		return e.modName, true
	}
	if i := strings.LastIndexByte(e.modName, '.'); i >= 0 {
		return e.modName[:i], true
	}
	return "", true
}

// hasModuleStar reports whether the module body contains a `from m import *`
// at module level. CPython compiles every free name in such a module with
// LOAD_NAME, since the star can introduce any name, so an otherwise unknown
// module-scope read defers to runtime rather than failing the compile.
func hasModuleStar(body []frontend.Stmt) bool {
	found := false
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.ImportFrom:
				if s.Star {
					found = true
				}
			case *frontend.If:
				walk(s.Body)
				walk(s.Else)
			case *frontend.While:
				walk(s.Body)
				walk(s.Else)
			case *frontend.For:
				walk(s.Body)
				walk(s.Else)
			case *frontend.With:
				walk(s.Body)
			case *frontend.Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(body)
	return found
}

// starModule resolves the module a `from ... import *` names against this
// module's package. ok is false when the form is impossible (a relative star
// with no parent or one reaching past the package root), the same conditions
// that make an ordinary relative import raise.
func (e *emitter) starModule(s *frontend.ImportFrom) (string, bool) {
	if s.Level == 0 {
		return s.Module, true
	}
	pack, known := e.selfPackage()
	if !known || pack == "" {
		return "", false
	}
	return RelativeName(pack, s.Level, s.Module)
}

// collectStarNames adds every name a module-level `from m import *` binds to
// out, walking module-level blocks but not def or class bodies (a star there
// is rejected at lowering). It uses the resolved module's export list; a star
// whose module does not resolve or was not compiled binds nothing statically.
func (e *emitter) collectStarNames(body []frontend.Stmt, out map[string]bool) {
	var walk func(list []frontend.Stmt)
	walk = func(list []frontend.Stmt) {
		for _, s := range list {
			switch s := s.(type) {
			case *frontend.ImportFrom:
				if !s.Star {
					continue
				}
				module, ok := e.starModule(s)
				if !ok {
					continue
				}
				exp, ok := e.stars[module]
				if !ok {
					continue
				}
				for _, n := range exp.All {
					out[n] = true
				}
				for _, n := range exp.Names {
					out[n] = true
				}
			case *frontend.If:
				walk(s.Body)
				walk(s.Else)
			case *frontend.While:
				walk(s.Body)
				walk(s.Else)
			case *frontend.For:
				walk(s.Body)
				walk(s.Else)
			case *frontend.With:
				walk(s.Body)
			case *frontend.Try:
				walk(s.Body)
				for _, h := range s.Handlers {
					walk(h.Body)
				}
				walk(s.OrElse)
				walk(s.Final)
			case *frontend.Match:
				for _, c := range s.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(body)
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
