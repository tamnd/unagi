package frontend

import (
	"strconv"
	"strings"
	"testing"
)

// The dumper below renders the AST as compact s-expressions so the tables
// can assert on shape without drowning in struct literals.

var binNames = map[BinKind]string{
	BinAdd: "+", BinSub: "-", BinMul: "*", BinDiv: "/",
	BinFloorDiv: "//", BinMod: "%", BinPow: "**",
}

var cmpNames = map[CmpKind]string{
	CmpEq: "==", CmpNe: "!=", CmpLt: "<", CmpLe: "<=", CmpGt: ">", CmpGe: ">=",
	CmpIn: "in", CmpNotIn: "not-in", CmpIs: "is", CmpIsNot: "is-not",
}

func dumpMod(m *Module) string {
	parts := make([]string, len(m.Body))
	for i, s := range m.Body {
		parts[i] = ds(s)
	}
	return strings.Join(parts, " ")
}

func dbody(body []Stmt) string {
	parts := make([]string, len(body))
	for i, s := range body {
		parts[i] = ds(s)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func ds(s Stmt) string {
	switch s := s.(type) {
	case *ExprStmt:
		return "(expr " + de(s.X) + ")"
	case *Assign:
		parts := make([]string, 0, len(s.Targets)+1)
		for _, t := range s.Targets {
			parts = append(parts, de(t))
		}
		parts = append(parts, de(s.Value))
		return "(= " + strings.Join(parts, " ") + ")"
	case *AugAssign:
		return "(" + binNames[s.Op] + "= " + de(s.Target) + " " + de(s.Value) + ")"
	case *If:
		return "(if " + de(s.Cond) + " " + dbody(s.Body) + " " + dbody(s.Else) + ")"
	case *While:
		return "(while " + de(s.Cond) + " " + dbody(s.Body) + " " + dbody(s.Else) + ")"
	case *For:
		return "(for " + de(s.Target) + " " + de(s.Iter) + " " + dbody(s.Body) + " " + dbody(s.Else) + ")"
	case *FuncDef:
		return "(def " + s.Name + " (" + strings.Join(s.Params, " ") + ") " + dbody(s.Body) + ")"
	case *Return:
		if s.Value == nil {
			return "(return)"
		}
		return "(return " + de(s.Value) + ")"
	case *Pass:
		return "(pass)"
	case *Break:
		return "(break)"
	case *Continue:
		return "(continue)"
	}
	return "?stmt"
}

func de(e Expr) string {
	switch e := e.(type) {
	case *Name:
		return e.Id
	case *IntLit:
		return e.Text
	case *FloatLit:
		return strconv.FormatFloat(e.Val, 'g', -1, 64)
	case *StrLit:
		return strconv.Quote(e.Val)
	case *BoolLit:
		if e.Val {
			return "True"
		}
		return "False"
	case *NoneLit:
		return "None"
	case *ListLit:
		return dexprs("list", e.Elts)
	case *TupleLit:
		return dexprs("tuple", e.Elts)
	case *DictLit:
		parts := []string{"dict"}
		for i := range e.Keys {
			parts = append(parts, "("+de(e.Keys[i])+" "+de(e.Vals[i])+")")
		}
		return "(" + strings.Join(parts, " ") + ")"
	case *BinOp:
		return "(" + binNames[e.Op] + " " + de(e.Left) + " " + de(e.Right) + ")"
	case *UnaryOp:
		name := map[UnaryKind]string{UnaryNeg: "neg", UnaryPos: "pos", UnaryNot: "not"}[e.Op]
		return "(" + name + " " + de(e.X) + ")"
	case *BoolOp:
		name := "and"
		if e.Kind == BoolOr {
			name = "or"
		}
		return dexprs(name, e.Values)
	case *Compare:
		parts := []string{"cmp", de(e.Left)}
		for i, op := range e.Ops {
			parts = append(parts, cmpNames[op], de(e.Rights[i]))
		}
		return "(" + strings.Join(parts, " ") + ")"
	case *Call:
		return dexprs("call", append([]Expr{e.Fn}, e.Args...))
	case *Attribute:
		return "(. " + de(e.X) + " " + e.Name + ")"
	case *Subscript:
		return "([] " + de(e.X) + " " + de(e.Index) + ")"
	}
	return "?expr"
}

func dexprs(name string, exprs []Expr) string {
	parts := []string{name}
	for _, e := range exprs {
		parts = append(parts, de(e))
	}
	return "(" + strings.Join(parts, " ") + ")"
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"empty module", "", ""},
		{"expr statement", "x", "(expr x)"},
		{"assignment", "x = 1", "(= x 1)"},
		{"chained assignment", "x = y = z", "(= x y z)"},
		{"tuple swap", "a, b = b, a", "(= (tuple a b) (tuple b a))"},
		{"subscript target", "a[0] = 1", "(= ([] a 0) 1)"},
		{"aug ops", "x += 1; x -= 2; x *= 3; x /= 4; x //= 5; x %= 6; x **= 7",
			"(+= x 1) (-= x 2) (*= x 3) (/= x 4) (//= x 5) (%= x 6) (**= x 7)"},
		{"aug subscript", "a[i] -= 2", "(-= ([] a i) 2)"},
		{"mul binds tighter", "1 + 2 * 3", "(expr (+ 1 (* 2 3)))"},
		{"parens override", "(1 + 2) * 3", "(expr (* (+ 1 2) 3))"},
		{"add left assoc", "1 + 2 - 3", "(expr (- (+ 1 2) 3))"},
		{"term left assoc", "6 / 3 // 2 % 2", "(expr (% (// (/ 6 3) 2) 2))"},
		{"pow beats unary minus", "-2 ** 2", "(expr (neg (** 2 2)))"},
		{"pow with unary right", "2 ** -1", "(expr (** 2 (neg 1)))"},
		{"pow right assoc", "2 ** 3 ** 2", "(expr (** 2 (** 3 2)))"},
		{"unary chain", "--x", "(expr (neg (neg x)))"},
		{"unary plus", "+x", "(expr (pos x))"},
		{"bool precedence", "not a or b and c", "(expr (or (not a) (and b c)))"},
		{"or flattens", "a or b or c", "(expr (or a b c))"},
		{"chained comparison", "1 < x <= 10", "(expr (cmp 1 < x <= 10))"},
		{"in", "a in b", "(expr (cmp a in b))"},
		{"not in", "a not in b", "(expr (cmp a not-in b))"},
		{"is", "a is b", "(expr (cmp a is b))"},
		{"is not", "a is not c", "(expr (cmp a is-not c))"},
		{"not of comparison", "not a == b", "(expr (not (cmp a == b)))"},
		{"postfix chain", "f(1, 2)(3).attr[0]", "(expr ([] (. (call (call f 1 2) 3) attr) 0))"},
		{"attribute chain", "x.y.z", "(expr (. (. x y) z))"},
		{"call no args", "f()", "(expr (call f))"},
		{"call trailing comma", "f(a,)", "(expr (call f a))"},
		{"tuple subscript", "a[1, 2]", "(expr ([] a (tuple 1 2)))"},
		{"bare tuple", "1, 2, 3", "(expr (tuple 1 2 3))"},
		{"bare one tuple", "1,", "(expr (tuple 1))"},
		{"paren one tuple", "(1,)", "(expr (tuple 1))"},
		{"paren grouping only", "(1)", "(expr 1)"},
		{"empty tuple", "()", "(expr (tuple))"},
		{"list", "[1, 2, 3]", "(expr (list 1 2 3))"},
		{"empty list", "[]", "(expr (list))"},
		{"list trailing comma", "[1, 2,]", "(expr (list 1 2))"},
		{"empty dict", "{}", "(expr (dict))"},
		{"dict", "{1: 'a', 'b': 2,}", `(expr (dict (1 "a") ("b" 2)))`},
		{"string concat", `'a' 'b' "c"`, `(expr "abc")`},
		{"string concat across lines", "x = ('a'\n'b')", `(= x "ab")`},
		{"semicolons", "True; False; None", "(expr True) (expr False) (expr None)"},
		{"float value", "1.5e3", "(expr 1500)"},
		{"hex assignment", "x = 0xDEADBEEF", "(= x 3735928559)"},
		{"def", "def add(a, b):\n    return a + b\n", "(def add (a b) [(return (+ a b))])"},
		{"def bare return", "def f():\n    return\n", "(def f () [(return)])"},
		{"def trailing comma", "def f(a, b,):\n    pass\n", "(def f (a b) [(pass)])"},
		{"return tuple", "return 1, 2", "(return (tuple 1 2))"},
		{"if elif else", "if a:\n    x = 1\nelif b:\n    y = 2\nelse:\n    z = 3\n",
			"(if a [(= x 1)] [(if b [(= y 2)] [(= z 3)])])"},
		{"one line if chain", "if a: pass\nelif b: pass\nelse: pass\n",
			"(if a [(pass)] [(if b [(pass)] [(pass)])])"},
		{"while else", "while x > 0:\n    x -= 1\nelse:\n    pass\n",
			"(while (cmp x > 0) [(-= x 1)] [(pass)])"},
		{"while break continue", "while True:\n    break\n    continue\n",
			"(while True [(break) (continue)] [])"},
		{"for tuple target else", "for a, b in pairs:\n    total += a\nelse:\n    pass\n",
			"(for (tuple a b) pairs [(+= total a)] [(pass)])"},
		{"for paren target", "for (a, b) in pairs: pass", "(for (tuple a b) pairs [(pass)] [])"},
		{"for bare tuple iter", "for x in 1, 2: pass", "(for x (tuple 1 2) [(pass)] [])"},
		{"one line suite", "if x: a = 1; b = 2", "(if x [(= a 1) (= b 2)] [])"},
		{"nested compound", "def f(n):\n    while n:\n        if n % 2 == 0:\n            n = n // 2\n        else:\n            n = 3 * n + 1\n    return n\n",
			"(def f (n) [(while n [(if (cmp (% n 2) == 0) [(= n (// n 2))] [(= n (+ (* 3 n) 1))])] []) (return n)])"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Parse([]byte(tt.src), "test.py")
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.src, err)
			}
			if got := dumpMod(m); got != tt.want {
				t.Errorf("Parse(%q)\n got  %s\n want %s", tt.src, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{"class", "class A: pass", "class definitions are not supported yet"},
		{"import", "import os", "import statements are not supported yet"},
		{"from import", "from os import path", "from imports are not supported yet"},
		{"try", "try:\n    pass\n", "try statements are not supported yet"},
		{"with", "with open(f) as g: pass", "with statements are not supported yet"},
		{"del", "del x", "del statements are not supported yet"},
		{"raise", "raise ValueError", "raise statements are not supported yet"},
		{"assert", "assert x", "assert statements are not supported yet"},
		{"global", "global x", "global statements are not supported yet"},
		{"nonlocal", "nonlocal x", "nonlocal statements are not supported yet"},
		{"async def", "async def f(): pass", "async is not supported yet"},
		{"lambda", "x = lambda a: a", "lambda expressions are not supported yet"},
		{"yield", "yield 1", "yield expressions are not supported yet"},
		{"await", "x = await f()", "await is not supported yet"},
		{"conditional expr", "x = 1 if y else 2", "conditional expressions are not supported yet"},
		{"list comprehension", "[x for x in y]", "list comprehensions are not supported yet"},
		{"dict comprehension", "{k: v for k in y}", "dict comprehensions are not supported yet"},
		{"set comprehension", "{x for x in y}", "set comprehensions are not supported yet"},
		{"generator expr", "(x for x in y)", "generator expressions are not supported yet"},
		{"generator arg", "f(x for x in y)", "generator expressions are not supported yet"},
		{"set literal", "{1, 2}", "set literals are not supported yet"},
		{"single set literal", "{1}", "set literals are not supported yet"},
		{"dict unpacking", "{**a}", "dict unpacking is not supported yet"},
		{"slice", "a[1:2]", "slices are not supported yet"},
		{"open slice", "a[:2]", "slices are not supported yet"},
		{"keyword argument", "f(a=1)", "keyword arguments are not supported yet"},
		{"star arg", "f(*a)", "'*' argument unpacking is not supported yet"},
		{"double star arg", "f(**a)", "'**' argument unpacking is not supported yet"},
		{"default parameter", "def f(a=1): pass", "default parameter values are not supported yet"},
		{"star parameter", "def f(*args): pass", "star parameters (*args) are not supported yet"},
		{"kw parameter", "def f(**kw): pass", "keyword parameters (**kwargs) are not supported yet"},
		{"param annotation", "def f(a: int): pass", "parameter annotations are not supported yet"},
		{"positional marker", "def f(a, /): pass", "positional-only parameter markers are not supported yet"},
		{"duplicate param", "def f(a, a): pass", "duplicate argument 'a' in function definition"},
		{"bad param", "def f(1): pass", "expected parameter name"},
		{"variable annotation", "x: int = 1", "variable annotations are not supported yet"},
		{"match statement", "match x:\n    case 1:\n        pass\n", "match statements are not supported yet"},
		{"assign to int", "1 = x", "cannot assign to literal"},
		{"assign to string", "'s' = x", "cannot assign to literal"},
		{"assign to True", "True = 1", "cannot assign to True"},
		{"assign to None", "None = 1", "cannot assign to None"},
		{"assign to call", "f() = 1", "cannot assign to function call"},
		{"assign to expression", "a + b = 1", "cannot assign to expression"},
		{"assign in tuple", "a, 1 = x", "cannot assign to literal"},
		{"list target", "[a] = x", "list assignment targets are not supported yet"},
		{"attribute target", "a.b = 1", "attribute assignment targets are not supported yet"},
		{"starred target", "*a, b = c", "starred expressions are not supported yet"},
		{"aug tuple target", "a, b += 1", "'tuple' is an illegal expression for augmented assignment"},
		{"aug literal target", "1 += 1", "illegal expression for augmented assignment"},
		{"for subscript target", "for a[0] in x: pass", "for loop target must be a name or tuple of names"},
		{"for literal target", "for 1 in x: pass", "for loop target must be a name or tuple of names"},
		{"missing block", "if x:\npass", "expected an indented block"},
		{"unexpected indent first line", "  x = 1", "unexpected indent"},
		{"unexpected indent later", "x = 1\n    y = 2\n", "unexpected indent"},
		{"two exprs on a line", "x 1", "invalid syntax"},
		{"dangling operator", "x = 1 +", "invalid syntax"},
		{"missing colon", "if x\n    pass\n", "expected ':'"},
		{"lexer error surfaces", "x = 0x", "invalid hexadecimal literal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.src), "test.py")
			if err == nil {
				t.Fatalf("Parse(%q): expected error containing %q, got none", tt.src, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Parse(%q)\n got  %v\n want substring %q", tt.src, err, tt.wantErr)
			}
		})
	}
}

func TestParseErrorFormat(t *testing.T) {
	_, err := Parse([]byte("1 = x"), "main.py")
	if err == nil {
		t.Fatal("expected error")
	}
	want := "main.py:1:1: SyntaxError: cannot assign to literal"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestParsePositions(t *testing.T) {
	m, err := Parse([]byte("x = 1\ny = f(2)\n"), "test.py")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Body[0].Span(); got != (Pos{Line: 1, Col: 1}) {
		t.Errorf("stmt 0 span %+v", got)
	}
	if got := m.Body[1].Span(); got != (Pos{Line: 2, Col: 1}) {
		t.Errorf("stmt 1 span %+v", got)
	}
	call := m.Body[1].(*Assign).Value.(*Call)
	if got := call.Args[0].Span(); got != (Pos{Line: 2, Col: 7}) {
		t.Errorf("call arg span %+v", got)
	}
}
