// Package frontend turns Python source into an AST: a lexer that produces the
// logical token stream (NEWLINE, INDENT, DEDENT and all), a recursive-descent
// parser over it, and the AST types below. The M0 surface is the statement and
// expression subset the emitter can lower; the parser grows toward the full
// 3.14 grammar milestone by milestone, and anything it does not recognize is a
// SyntaxError with a CPython-shaped message, never a silent skip.
package frontend

import "strings"

// Pos is a position in the source file, 1-based like CPython's.
type Pos struct {
	Line int
	Col  int
}

// Node is anything with a source position.
type Node interface {
	Span() Pos
}

// Stmt is a statement node.
type Stmt interface {
	Node
	stmt()
}

// Expr is an expression node.
type Expr interface {
	Node
	expr()
}

// Module is one parsed source file.
type Module struct {
	Body []Stmt
	// EscapeWarnings are the invalid backslash-escape SyntaxWarnings the lexer
	// found, one per string literal, in source order. CPython 3.14 prints these
	// at compile time; the lowering replays them before the module body runs.
	EscapeWarnings []EscapeWarning
}

// EscapeWarning records one invalid escape sequence (a backslash followed by a
// character that is not a recognized escape). Char is the offending character
// after the backslash, shown verbatim in the warning text.
type EscapeWarning struct {
	Line int
	Col  int
	Char string
}

// --- statements ---

// ExprStmt is an expression evaluated for effect, like a bare call.
type ExprStmt struct {
	Pos_ Pos
	X    Expr
}

// Assign is `a = b` and chained `a = b = c`; Targets holds every left side in
// source order. Targets are Name, Subscript, or TupleLit of those.
type Assign struct {
	Pos_    Pos
	Targets []Expr
	Value   Expr
}

// AugAssign is `a += b` and friends. Target is Name or Subscript.
type AugAssign struct {
	Pos_   Pos
	Target Expr
	Op     BinKind
	Value  Expr
}

// AnnAssign is `target: annotation` and `target: annotation = value` (PEP
// 526). Only a single Name, Attribute, or Subscript may be annotated. Value
// is nil for a bare annotation. Following PEP 649 the annotation expression is
// never evaluated, so the parser discards it and this node does not carry it.
type AnnAssign struct {
	Pos_   Pos
	Target Expr
	Value  Expr
}

// If is the full if/elif/else chain; elif nests as another If in Else.
type If struct {
	Pos_ Pos
	Cond Expr
	Body []Stmt
	Else []Stmt
}

// While is a while loop with its optional else block.
type While struct {
	Pos_ Pos
	Cond Expr
	Body []Stmt
	Else []Stmt
}

// For is `for target in iter:` with its optional else block. Target is Name
// or TupleLit of Names.
type For struct {
	Pos_   Pos
	Target Expr
	Iter   Expr
	Body   []Stmt
	Else   []Stmt
}

// WithItem is one `context as target` clause of a with statement. Target is
// nil when the clause has no `as` binding, and otherwise is an assignment
// target (Name, TupleLit, Attribute, or Subscript) like any other.
type WithItem struct {
	Context Expr
	Target  Expr
}

// With is `with item, ...: body`. Multiple items behave exactly like nested
// single-item with statements: earlier managers enter first and exit last,
// and a later manager failing to enter still exits the earlier ones.
type With struct {
	Pos_  Pos
	Items []WithItem
	Body  []Stmt
}

// Match is `match subject:` with one or more case clauses (PEP 634). Subject
// is the value matched against each case in written order.
type Match struct {
	Pos_    Pos
	Subject Expr
	Cases   []MatchCase
}

// MatchCase is one `case pattern [if guard]:` clause. Guard is nil when the
// clause has no `if` condition.
type MatchCase struct {
	Pos_    Pos
	Pattern Pattern
	Guard   Expr
	Body    []Stmt
}

// ParamKind classifies a formal parameter.
type ParamKind int

const (
	ParamPlain    ParamKind = iota // positional-or-keyword
	ParamPosOnly                   // declared before the / marker
	ParamStar                      // *args
	ParamKwOnly                    // declared after * or a bare *
	ParamStarStar                  // **kwargs
)

// Param is one formal parameter. Default is nil when the parameter has
// none. The parser enforces CPython's ordering rules (posonly, plain,
// star, kwonly, starstar; no non-default after default within a group)
// so lowering can trust the layout.
type Param struct {
	Pos_    Pos
	Name    string
	Kind    ParamKind
	Default Expr
}

// FuncDef is `def name(params):`. Decorators holds the decorator expressions
// in written (top to bottom) order, empty for an undecorated def.
type FuncDef struct {
	Pos_       Pos
	Name       string
	Params     []Param
	Body       []Stmt
	Decorators []Expr
}

// ClassDef is `class Name(bases): body`. Bases holds the base-class
// expressions in written order; an empty slice is the bare `class Name:`
// form. Keywords holds the class keyword arguments (`metaclass=M`, and the
// names passed on to __init_subclass__) in written order. The body carries
// method defs and class-variable assignments. Decorators holds the decorator
// expressions in written order.
type ClassDef struct {
	Pos_       Pos
	Name       string
	Bases      []Expr
	Keywords   []ClassKeyword
	Body       []Stmt
	Decorators []Expr
}

// ClassKeyword is one `name=value` in a class header, such as the metaclass
// argument or a name handed to __init_subclass__.
type ClassKeyword struct {
	Name  string
	Value Expr
}

// Try is the full try/except/else/finally statement. A try with no handlers
// carries only Final (the try/finally form); the parser enforces that at
// least one of Handlers and Final is present.
type Try struct {
	Pos_     Pos
	Body     []Stmt
	Handlers []*ExceptHandler
	OrElse   []Stmt
	Final    []Stmt
	// IsStar marks a try whose handlers are except* (PEP 654). The parser
	// enforces that every clause on one try agrees, so the flag is set once
	// from the first handler and describes all of them.
	IsStar bool
}

// ExceptHandler is one except clause. Type is nil for the bare `except:`
// form and Name is empty when there is no `as` binding. The grammar allows
// any expression as the matcher; the emitter enforces what it can lower.
type ExceptHandler struct {
	Pos_ Pos
	Type Expr
	Name string
	Body []Stmt
}

func (h *ExceptHandler) Span() Pos { return h.Pos_ }

// Raise is `raise`, `raise exc`, and `raise exc from cause`. Exc is nil for
// the bare re-raise form; Cause is nil when there is no from clause, and a
// NoneLit for the explicit `from None`.
type Raise struct {
	Pos_  Pos
	Exc   Expr
	Cause Expr
}

// Assert is `assert test` and `assert test, msg`; Msg is nil for the bare
// form.
type Assert struct {
	Pos_ Pos
	Test Expr
	Msg  Expr
}

// Return is `return` or `return value`; Value is nil for the bare form.
type Return struct {
	Pos_  Pos
	Value Expr
}

// Pass is `pass`.
type Pass struct {
	Pos_ Pos
}

// Break is `break`.
type Break struct {
	Pos_ Pos
}

// Continue is `continue`.
type Continue struct {
	Pos_ Pos
}

// Del is `del a, b[k]`. Targets are Name or Subscript (with or without a
// slice); the parser rejects anything else with CPython's message.
type Del struct {
	Pos_    Pos
	Targets []Expr
}

// Global is `global a, b`: inside a function the listed names read and
// write module scope. At module scope the statement is legal and changes
// nothing. The parser enforces the conflict rules after the parse.
type Global struct {
	Pos_  Pos
	Names []string
}

// Nonlocal is `nonlocal a, b`: inside a function the listed names read and
// write the nearest enclosing function scope that binds them. The parser
// enforces the conflict and binding rules after the parse.
type Nonlocal struct {
	Pos_  Pos
	Names []string
}

// ImportAlias is one name in an import statement. Name is the dotted module
// path as written (or the attribute name in a from import), and As is the
// binding name when an `as` clause renames it, empty otherwise.
type ImportAlias struct {
	Pos_ Pos
	Name string
	As   string
}

// Bound is the name the alias binds in the importing scope: the As name when
// present, otherwise the first segment of the dotted path (import a.b binds
// a) or the plain name for a from import.
func (a ImportAlias) Bound() string {
	if a.As != "" {
		return a.As
	}
	if i := strings.IndexByte(a.Name, '.'); i >= 0 {
		return a.Name[:i]
	}
	return a.Name
}

// Import is `import a, b.c as d`.
type Import struct {
	Pos_  Pos
	Names []ImportAlias
}

// ImportFrom is `from ...mod import a, b as c` or `from mod import *`.
// Module is the dotted path after the relative dots and may be empty for a
// pure-relative form like `from . import x`; Level counts the leading dots,
// zero for an absolute import. Star marks the `*` form, which carries no
// aliases.
type ImportFrom struct {
	Pos_   Pos
	Module string
	Level  int
	Names  []ImportAlias
	Star   bool
}

func (s *Del) Span() Pos        { return s.Pos_ }
func (s *Global) Span() Pos     { return s.Pos_ }
func (s *Nonlocal) Span() Pos   { return s.Pos_ }
func (s *Import) Span() Pos     { return s.Pos_ }
func (s *ImportFrom) Span() Pos { return s.Pos_ }
func (s *Try) Span() Pos        { return s.Pos_ }
func (s *Raise) Span() Pos      { return s.Pos_ }
func (s *Assert) Span() Pos     { return s.Pos_ }
func (s *ExprStmt) Span() Pos   { return s.Pos_ }
func (s *Assign) Span() Pos     { return s.Pos_ }
func (s *AugAssign) Span() Pos  { return s.Pos_ }
func (s *AnnAssign) Span() Pos  { return s.Pos_ }
func (s *If) Span() Pos         { return s.Pos_ }
func (s *While) Span() Pos      { return s.Pos_ }
func (s *For) Span() Pos        { return s.Pos_ }
func (s *With) Span() Pos       { return s.Pos_ }
func (s *Match) Span() Pos      { return s.Pos_ }
func (s *FuncDef) Span() Pos    { return s.Pos_ }
func (s *ClassDef) Span() Pos   { return s.Pos_ }
func (s *Return) Span() Pos     { return s.Pos_ }
func (s *Pass) Span() Pos       { return s.Pos_ }
func (s *Break) Span() Pos      { return s.Pos_ }
func (s *Continue) Span() Pos   { return s.Pos_ }

func (*Del) stmt()        {}
func (*Global) stmt()     {}
func (*Nonlocal) stmt()   {}
func (*Import) stmt()     {}
func (*ImportFrom) stmt() {}
func (*Try) stmt()        {}
func (*Raise) stmt()      {}
func (*Assert) stmt()     {}
func (*ExprStmt) stmt()   {}
func (*Assign) stmt()     {}
func (*AugAssign) stmt()  {}
func (*AnnAssign) stmt()  {}
func (*If) stmt()         {}
func (*While) stmt()      {}
func (*For) stmt()        {}
func (*With) stmt()       {}
func (*Match) stmt()      {}
func (*FuncDef) stmt()    {}
func (*ClassDef) stmt()   {}
func (*Return) stmt()     {}
func (*Pass) stmt()       {}
func (*Break) stmt()      {}
func (*Continue) stmt()   {}

// --- expressions ---

// Name is an identifier reference.
type Name struct {
	Pos_ Pos
	Id   string
}

// IntLit keeps the literal text so magnitude is never lost before the emitter
// decides how to represent it. The text is the normalized decimal form (the
// lexer folds 0x/0o/0b and underscores away).
type IntLit struct {
	Pos_ Pos
	Text string
}

// FloatLit is a float literal; Val carries the parsed value.
type FloatLit struct {
	Pos_ Pos
	Val  float64
}

// ImagLit is an imaginary literal like 2j; Val carries the coefficient, which
// the lowering pairs with a zero real part to build the complex.
type ImagLit struct {
	Pos_ Pos
	Val  float64
}

// StrLit is a string literal after quote and escape processing. Adjacent
// literals are already concatenated by the parser.
type StrLit struct {
	Pos_ Pos
	Val  string
}

// BytesLit is a bytes literal after quote and escape processing. Val holds
// the decoded bytes, one Go byte per byte value. Adjacent bytes literals are
// already concatenated by the parser.
type BytesLit struct {
	Pos_ Pos
	Val  string
}

// BoolLit is True or False.
type BoolLit struct {
	Pos_ Pos
	Val  bool
}

// NoneLit is None.
type NoneLit struct {
	Pos_ Pos
}

// EllipsisLit is the `...` literal, the Ellipsis singleton.
type EllipsisLit struct {
	Pos_ Pos
}

// ListLit is `[a, b, c]`.
type ListLit struct {
	Pos_ Pos
	Elts []Expr
}

// TupleLit is `(a, b)` and the bare `a, b` form; parenthesization is not
// recorded.
type TupleLit struct {
	Pos_ Pos
	Elts []Expr
}

// DictLit is `{k: v, ...}`; Keys and Vals run in parallel.
type DictLit struct {
	Pos_ Pos
	Keys []Expr
	Vals []Expr
}

// SetLit is `{a, b, ...}` with at least one element; empty braces are always
// a dict.
type SetLit struct {
	Pos_ Pos
	Elts []Expr
}

// CompKind picks the container a comprehension builds.
type CompKind int

const (
	CompList CompKind = iota
	CompSet
	CompDict
	CompGen
)

// CompClause is one `for target in iter` leg with its trailing `if`
// conditions. A comprehension carries one or more in source order.
type CompClause struct {
	Pos_   Pos
	Target Expr
	Iter   Expr
	Ifs    []Expr
}

// Comp is a list, set, or dict comprehension, or a generator expression when
// Kind is CompGen. Elt is the element, or the key when Kind is CompDict, with
// Val carrying the value.
type Comp struct {
	Pos_    Pos
	Kind    CompKind
	Elt     Expr
	Val     Expr
	Clauses []CompClause
}

// BinKind is an arithmetic or bitwise binary operator.
type BinKind int

const (
	BinAdd BinKind = iota
	BinSub
	BinMul
	BinDiv      // /
	BinFloorDiv // //
	BinMod
	BinPow
	BinBitOr  // |
	BinBitXor // ^
	BinBitAnd // &
	BinLShift // <<
	BinRShift // >>
	BinMatMul // @
)

// BinOp is `left op right` for the arithmetic and bitwise operators.
type BinOp struct {
	Pos_  Pos
	Left  Expr
	Op    BinKind
	Right Expr
}

// UnaryKind is a unary operator.
type UnaryKind int

const (
	UnaryNeg UnaryKind = iota
	UnaryPos
	UnaryNot
	UnaryInvert // ~
)

// UnaryOp is `-x`, `+x`, `~x`, or `not x`.
type UnaryOp struct {
	Pos_ Pos
	Op   UnaryKind
	X    Expr
}

// BoolKind selects and/or.
type BoolKind int

const (
	BoolAnd BoolKind = iota
	BoolOr
)

// BoolOp is `a and b and c`; Values holds two or more operands.
type BoolOp struct {
	Pos_   Pos
	Kind   BoolKind
	Values []Expr
}

// CmpKind is a comparison operator.
type CmpKind int

const (
	CmpEq CmpKind = iota
	CmpNe
	CmpLt
	CmpLe
	CmpGt
	CmpGe
	CmpIn
	CmpNotIn
	CmpIs
	CmpIsNot
)

// Compare is a possibly chained comparison: `a < b <= c` has Left a, Ops
// [CmpLt, CmpLe], Rights [b, c].
type Compare struct {
	Pos_   Pos
	Left   Expr
	Ops    []CmpKind
	Rights []Expr
}

// Call is `fn(args)`. M0 arguments are positional only; the parser rejects
// keyword arguments and star-unpacking with a clear message.
// Arg is one call-site argument. Name is the keyword name, empty for a
// positional argument. Star is 0 for a plain argument, 1 for *sequence
// unpacking, and 2 for **mapping unpacking.
type Arg struct {
	Pos_  Pos
	Name  string
	Star  int
	Value Expr
}

type Call struct {
	Pos_ Pos
	Fn   Expr
	Args []Arg
}

// Attribute is `x.name`.
type Attribute struct {
	Pos_ Pos
	X    Expr
	Name string
}

// Subscript is `x[index]`. Index is a SliceExpr when the brackets hold a
// colon form.
type Subscript struct {
	Pos_  Pos
	X     Expr
	Index Expr
}

// SliceExpr is the `lo:hi:step` form inside subscript brackets. Any of the
// three parts may be nil. It appears only as a Subscript index.
type SliceExpr struct {
	Pos_ Pos
	Lo   Expr
	Hi   Expr
	Step Expr
}

// IfExp is the conditional expression `then if cond else else_`. Exactly one
// arm is evaluated, after the condition.
type IfExp struct {
	Pos_ Pos
	Cond Expr
	Then Expr
	Else Expr
}

// NamedExpr is the walrus `name := value`. CPython only allows a plain name
// as the target, so the target is a string, not an Expr.
type NamedExpr struct {
	Pos_   Pos
	Target string
	Value  Expr
}

// Starred is `*name` inside an assignment or for-loop target tuple. The
// parser enforces at most one per target list, per CPython.
type Starred struct {
	Pos_ Pos
	X    Expr
}

// Lambda is `lambda params: body`. The parameter grammar is the def one
// without annotations or parentheses; the body is a single expression, so
// the node needs no statement list.
type Lambda struct {
	Pos_   Pos
	Params []Param
	Body   Expr
}

// Yield is a `yield` or `yield from` expression. Value is the yielded
// expression, nil for a bare `yield`. From is true for `yield from`, whose
// value is the iterable or awaitable being delegated to. A yield makes its
// enclosing function a generator; the parser rejects one outside a function.
type Yield struct {
	Pos_  Pos
	Value Expr
	From  bool
}

func (e *Yield) Span() Pos { return e.Pos_ }
func (*Yield) expr()       {}

// FStr is an f-string after parsing: literal text runs interleaved with
// interpolations, in source order. Adjacent string and f-string literals in
// a concatenation are already merged into one FStr by the parser.
type FStr struct {
	Pos_  Pos
	Parts []FPart
}

// FPart is one piece of an f-string: FText or FInterp.
type FPart interface {
	fpart()
}

// FText is a literal text run, with doubled braces already collapsed and
// escapes processed.
type FText struct {
	Text string
}

// FInterp is one {expression} interpolation. Conv is 0 when absent or one of
// 's', 'r', 'a'. Spec is the format spec after the colon, itself a list of
// text runs and nested interpolations (PEP 701 allows `{x:{width}}`); a plain
// `{x:>6}` is a single FText and `{x:}` is empty. HasSpec tells the empty spec
// apart from no spec at all, which matters for the self-documenting form. Eq
// holds the verbatim source text through the equals sign (whitespace included)
// for `{x=}`, empty otherwise.
type FInterp struct {
	Pos_    Pos
	X       Expr
	Conv    byte
	Spec    []FPart
	HasSpec bool
	Eq      string
}

func (*FText) fpart()   {}
func (*FInterp) fpart() {}

// FInterps returns every interpolation in the given f-string parts, including
// those nested inside a field's format spec, depth-first in source order. Name
// analysis, mangling, and yield detection use it so an expression inside a
// spec, as in f"{x:{w}}", is seen exactly like any other interpolation.
func FInterps(parts []FPart) []*FInterp {
	var out []*FInterp
	var walk func(ps []FPart)
	walk = func(ps []FPart) {
		for _, p := range ps {
			if in, ok := p.(*FInterp); ok {
				out = append(out, in)
				walk(in.Spec)
			}
		}
	}
	walk(parts)
	return out
}

func (e *Name) Span() Pos        { return e.Pos_ }
func (e *IntLit) Span() Pos      { return e.Pos_ }
func (e *FloatLit) Span() Pos    { return e.Pos_ }
func (e *ImagLit) Span() Pos     { return e.Pos_ }
func (e *StrLit) Span() Pos      { return e.Pos_ }
func (e *BytesLit) Span() Pos    { return e.Pos_ }
func (e *BoolLit) Span() Pos     { return e.Pos_ }
func (e *NoneLit) Span() Pos     { return e.Pos_ }
func (e *EllipsisLit) Span() Pos { return e.Pos_ }
func (e *ListLit) Span() Pos     { return e.Pos_ }
func (e *TupleLit) Span() Pos    { return e.Pos_ }
func (e *DictLit) Span() Pos     { return e.Pos_ }
func (e *SetLit) Span() Pos      { return e.Pos_ }
func (e *Comp) Span() Pos        { return e.Pos_ }
func (e *BinOp) Span() Pos       { return e.Pos_ }
func (e *UnaryOp) Span() Pos     { return e.Pos_ }
func (e *BoolOp) Span() Pos      { return e.Pos_ }
func (e *Compare) Span() Pos     { return e.Pos_ }
func (e *Call) Span() Pos        { return e.Pos_ }
func (e *Attribute) Span() Pos   { return e.Pos_ }
func (e *Subscript) Span() Pos   { return e.Pos_ }
func (e *SliceExpr) Span() Pos   { return e.Pos_ }
func (e *IfExp) Span() Pos       { return e.Pos_ }
func (e *NamedExpr) Span() Pos   { return e.Pos_ }
func (e *Starred) Span() Pos     { return e.Pos_ }
func (e *Lambda) Span() Pos      { return e.Pos_ }
func (e *FStr) Span() Pos        { return e.Pos_ }

func (*Name) expr()        {}
func (*IntLit) expr()      {}
func (*FloatLit) expr()    {}
func (*ImagLit) expr()     {}
func (*StrLit) expr()      {}
func (*BytesLit) expr()    {}
func (*BoolLit) expr()     {}
func (*NoneLit) expr()     {}
func (*EllipsisLit) expr() {}
func (*ListLit) expr()     {}
func (*TupleLit) expr()    {}
func (*DictLit) expr()     {}
func (*SetLit) expr()      {}
func (*Comp) expr()        {}
func (*BinOp) expr()       {}
func (*UnaryOp) expr()     {}
func (*BoolOp) expr()      {}
func (*Compare) expr()     {}
func (*Call) expr()        {}
func (*Attribute) expr()   {}
func (*Subscript) expr()   {}
func (*SliceExpr) expr()   {}
func (*IfExp) expr()       {}
func (*NamedExpr) expr()   {}
func (*Starred) expr()     {}
func (*Lambda) expr()      {}
func (*FStr) expr()        {}

// --- patterns ---

// Pattern is a case pattern (PEP 634). Patterns appear only inside a match
// statement's case clauses, never as expressions.
type Pattern interface {
	Node
	pattern()
}

// PatLiteral matches by value against a literal: a number, string, None,
// True, or False. Value is the literal expression; None/True/False compare by
// identity, everything else by equality.
type PatLiteral struct {
	Pos_  Pos
	Value Expr
}

// PatValue matches by equality against a dotted name, like `Color.RED`. Value
// is the attribute-access expression; a bare name is never a value pattern.
type PatValue struct {
	Pos_  Pos
	Value Expr
}

// PatCapture binds the subject to a name. Name "_" is the wildcard, which
// matches anything and binds nothing.
type PatCapture struct {
	Pos_ Pos
	Name string
}

// PatStar is `*name` inside a sequence pattern, binding the middle run as a
// list. Name "_" drops the run without binding.
type PatStar struct {
	Pos_ Pos
	Name string
}

// PatSequence matches a sequence subject element by element. Elts may hold at
// most one PatStar, which absorbs a variable-length run.
type PatSequence struct {
	Pos_ Pos
	Elts []Pattern
}

// PatMapping matches a mapping subject by key. Keys and Vals run in parallel;
// Rest is the `**name` capture for the remaining items, empty when absent.
type PatMapping struct {
	Pos_ Pos
	Keys []Expr
	Vals []Pattern
	Rest string
}

// PatClass matches `Cls(pos..., kw=...)`: an isinstance check plus positional
// sub-patterns via __match_args__ and keyword sub-patterns by attribute. Cls
// is the class expression, a name or dotted name.
type PatClass struct {
	Pos_     Pos
	Cls      Expr
	Pos      []Pattern
	KwNames  []string
	KwValues []Pattern
}

// PatOr is `a | b | c`; Alts holds two or more alternatives tried left to
// right. Every alternative binds the same set of names.
type PatOr struct {
	Pos_ Pos
	Alts []Pattern
}

// PatAs is `pattern as name`: match the inner pattern, then bind the subject
// to name. A bare `as name` with no inner pattern parses as a PatCapture.
type PatAs struct {
	Pos_    Pos
	Pattern Pattern
	Name    string
}

func (p *PatLiteral) Span() Pos  { return p.Pos_ }
func (p *PatValue) Span() Pos    { return p.Pos_ }
func (p *PatCapture) Span() Pos  { return p.Pos_ }
func (p *PatStar) Span() Pos     { return p.Pos_ }
func (p *PatSequence) Span() Pos { return p.Pos_ }
func (p *PatMapping) Span() Pos  { return p.Pos_ }
func (p *PatClass) Span() Pos    { return p.Pos_ }
func (p *PatOr) Span() Pos       { return p.Pos_ }
func (p *PatAs) Span() Pos       { return p.Pos_ }

func (*PatLiteral) pattern()  {}
func (*PatValue) pattern()    {}
func (*PatCapture) pattern()  {}
func (*PatStar) pattern()     {}
func (*PatSequence) pattern() {}
func (*PatMapping) pattern()  {}
func (*PatClass) pattern()    {}
func (*PatOr) pattern()       {}
func (*PatAs) pattern()       {}

// PatternNames returns the capture names a pattern binds, deduplicated and in
// no particular order, with the wildcard "_" excluded. Both scope analysis
// and lowering use it to know which locals a case introduces.
func PatternNames(p Pattern) []string {
	seen := map[string]bool{}
	var walk func(Pattern)
	add := func(name string) {
		if name != "" && name != "_" {
			seen[name] = true
		}
	}
	walk = func(p Pattern) {
		switch p := p.(type) {
		case *PatCapture:
			add(p.Name)
		case *PatStar:
			add(p.Name)
		case *PatAs:
			if p.Pattern != nil {
				walk(p.Pattern)
			}
			add(p.Name)
		case *PatSequence:
			for _, e := range p.Elts {
				walk(e)
			}
		case *PatMapping:
			for _, v := range p.Vals {
				walk(v)
			}
			add(p.Rest)
		case *PatClass:
			for _, sp := range p.Pos {
				walk(sp)
			}
			for _, sp := range p.KwValues {
				walk(sp)
			}
		case *PatOr:
			for _, alt := range p.Alts {
				walk(alt)
			}
		}
	}
	walk(p)
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	return out
}
