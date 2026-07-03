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
	BinBitOr: "|", BinBitXor: "^", BinBitAnd: "&",
	BinLShift: "<<", BinRShift: ">>",
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
		return "(def " + s.Name + " (" + strings.Join(dparams(s.Params), " ") + ") " + dbody(s.Body) + ")"
	case *Try:
		parts := []string{"try", dbody(s.Body)}
		for _, h := range s.Handlers {
			parts = append(parts, dhandler(h))
		}
		parts = append(parts, dbody(s.OrElse), dbody(s.Final))
		return "(" + strings.Join(parts, " ") + ")"
	case *Raise:
		out := "(raise"
		if s.Exc != nil {
			out += " " + de(s.Exc)
		}
		if s.Cause != nil {
			out += " from " + de(s.Cause)
		}
		return out + ")"
	case *Assert:
		if s.Msg == nil {
			return "(assert " + de(s.Test) + ")"
		}
		return "(assert " + de(s.Test) + " " + de(s.Msg) + ")"
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
	case *Del:
		return dexprs("del", s.Targets)
	}
	return "?stmt"
}

// dparams renders a parameter list back into source-like pieces. The / and
// bare * markers carry no Param of their own, so they are reconstructed from
// the kind sequence: / lands after the last posonly param, and a bare *
// lands before the first kwonly param when no *args produced one.
func dparams(params []Param) []string {
	lastPos := -1
	for i, pr := range params {
		if pr.Kind == ParamPosOnly {
			lastPos = i
		}
	}
	starSeen := false
	var parts []string
	for i, pr := range params {
		name := pr.Name
		switch pr.Kind {
		case ParamStar:
			name = "*" + name
			starSeen = true
		case ParamStarStar:
			name = "**" + name
		case ParamKwOnly:
			if !starSeen {
				parts = append(parts, "*")
				starSeen = true
			}
		}
		if pr.Default != nil {
			name += "=" + de(pr.Default)
		}
		parts = append(parts, name)
		if i == lastPos {
			parts = append(parts, "/")
		}
	}
	return parts
}

func dhandler(h *ExceptHandler) string {
	out := "(except"
	if h.Type != nil {
		out += " " + de(h.Type)
	}
	if h.Name != "" {
		out += " as " + h.Name
	}
	return out + " " + dbody(h.Body) + ")"
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
	case *SetLit:
		return dexprs("set", e.Elts)
	case *BinOp:
		return "(" + binNames[e.Op] + " " + de(e.Left) + " " + de(e.Right) + ")"
	case *UnaryOp:
		name := map[UnaryKind]string{UnaryNeg: "neg", UnaryPos: "pos", UnaryNot: "not", UnaryInvert: "inv"}[e.Op]
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
		parts := []string{"call", de(e.Fn)}
		for _, a := range e.Args {
			if a.Name != "" {
				parts = append(parts, a.Name+"="+de(a.Value))
			} else {
				parts = append(parts, de(a.Value))
			}
		}
		return "(" + strings.Join(parts, " ") + ")"
	case *Attribute:
		return "(. " + de(e.X) + " " + e.Name + ")"
	case *Subscript:
		return "([] " + de(e.X) + " " + de(e.Index) + ")"
	case *SliceExpr:
		return "(slice " + dopt(e.Lo) + " " + dopt(e.Hi) + " " + dopt(e.Step) + ")"
	case *IfExp:
		return "(ifexp " + de(e.Cond) + " " + de(e.Then) + " " + de(e.Else) + ")"
	case *NamedExpr:
		return "(:= " + e.Target + " " + de(e.Value) + ")"
	case *Starred:
		return "(* " + de(e.X) + ")"
	case *FStr:
		parts := []string{"fstr"}
		for _, part := range e.Parts {
			switch part := part.(type) {
			case *FText:
				parts = append(parts, strconv.Quote(part.Text))
			case *FInterp:
				parts = append(parts, dinterp(part))
			}
		}
		return "(" + strings.Join(parts, " ") + ")"
	}
	return "?expr"
}

// dinterp renders one f-string interpolation: the expression, then the
// =text, !conv, and :spec pieces in source order, each only when present.
func dinterp(in *FInterp) string {
	out := "(interp " + de(in.X)
	if in.Eq != "" {
		out += " =" + strconv.Quote(in.Eq)
	}
	if in.Conv != 0 {
		out += " !" + string(in.Conv)
	}
	if in.HasSpec {
		out += " :" + strconv.Quote(in.Spec)
	}
	return out + ")"
}

// dopt renders an optional slice part, with _ standing in for an omitted one.
func dopt(e Expr) string {
	if e == nil {
		return "_"
	}
	return de(e)
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
		{"bitwise aug ops", "x |= 1; x ^= 2; x &= 3; x <<= 4; x >>= 5",
			"(|= x 1) (^= x 2) (&= x 3) (<<= x 4) (>>= x 5)"},
		{"mul binds tighter", "1 + 2 * 3", "(expr (+ 1 (* 2 3)))"},
		{"parens override", "(1 + 2) * 3", "(expr (* (+ 1 2) 3))"},
		{"add left assoc", "1 + 2 - 3", "(expr (- (+ 1 2) 3))"},
		{"term left assoc", "6 / 3 // 2 % 2", "(expr (% (// (/ 6 3) 2) 2))"},
		{"pow beats unary minus", "-2 ** 2", "(expr (neg (** 2 2)))"},
		{"pow with unary right", "2 ** -1", "(expr (** 2 (neg 1)))"},
		{"pow right assoc", "2 ** 3 ** 2", "(expr (** 2 (** 3 2)))"},
		{"unary chain", "--x", "(expr (neg (neg x)))"},
		{"unary plus", "+x", "(expr (pos x))"},
		{"bitwise ladder", "1 | 2 ^ 3 & 4 << 5 + 6", "(expr (| 1 (^ 2 (& 3 (<< 4 (+ 5 6))))))"},
		{"bitor left assoc", "a | b | c", "(expr (| (| a b) c))"},
		{"bitxor left assoc", "a ^ b ^ c", "(expr (^ (^ a b) c))"},
		{"bitand left assoc", "a & b & c", "(expr (& (& a b) c))"},
		{"shift left assoc", "a << b >> c", "(expr (>> (<< a b) c))"},
		{"bitand beats equality", "a & b == c", "(expr (cmp (& a b) == c))"},
		{"bitor operands compared", "a | b < c | d", "(expr (cmp (| a b) < (| c d)))"},
		{"in of bitor", "1 in a | b", "(expr (cmp 1 in (| a b)))"},
		{"not of bitor", "not a | b", "(expr (not (| a b)))"},
		{"invert", "~x", "(expr (inv x))"},
		{"invert chain", "~~x", "(expr (inv (inv x)))"},
		{"invert of neg", "~-x", "(expr (inv (neg x)))"},
		{"neg of invert", "-~x", "(expr (neg (inv x)))"},
		{"pow beats invert", "~2 ** 2", "(expr (inv (** 2 2)))"},
		{"invert on pow right", "2 ** ~x", "(expr (** 2 (inv x)))"},
		{"invert binds tighter than mul", "~x * y", "(expr (* (inv x) y))"},
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
		{"set single", "{1}", "(expr (set 1))"},
		{"set multiple", "{1, 'a', x}", `(expr (set 1 "a" x))`},
		{"set trailing comma", "{1, 2,}", "(expr (set 1 2))"},
		{"set nested containers", "{(1, 2), [3]}", "(expr (set (tuple 1 2) (list 3)))"},
		{"set inside set", "{{1}}", "(expr (set (set 1)))"},
		{"set as dict value", "{1: {2}}", "(expr (dict (1 (set 2))))"},
		{"set expression element", "{a | b, ~c}", "(expr (set (| a b) (inv c)))"},
		{"set walrus in parens", "{(x := 1), x}", "(expr (set (:= x 1) x))"},
		{"set as assign value", "x = {1, 2}", "(= x (set 1 2))"},
		{"string concat", `'a' 'b' "c"`, `(expr "abc")`},
		{"string concat across lines", "x = ('a'\n'b')", `(= x "ab")`},
		{"semicolons", "True; False; None", "(expr True) (expr False) (expr None)"},
		{"float value", "1.5e3", "(expr 1500)"},
		{"hex assignment", "x = 0xDEADBEEF", "(= x 3735928559)"},
		{"def", "def add(a, b):\n    return a + b\n", "(def add (a b) [(return (+ a b))])"},
		{"def bare return", "def f():\n    return\n", "(def f () [(return)])"},
		{"def trailing comma", "def f(a, b,):\n    pass\n", "(def f (a b) [(pass)])"},
		{"def default", "def f(a, b=1): pass", "(def f (a b=1) [(pass)])"},
		{"def default complex exprs", "def f(a=[1, 2], b=x if y else z): pass",
			"(def f (a=(list 1 2) b=(ifexp y x z)) [(pass)])"},
		{"def default call expr", "def f(a=g(1) + 2): pass", "(def f (a=(+ (call g 1) 2)) [(pass)])"},
		{"def posonly", "def f(a, b, /, c): pass", "(def f (a b / c) [(pass)])"},
		{"def posonly only", "def f(a, /): pass", "(def f (a /) [(pass)])"},
		{"def posonly trailing comma", "def f(a, /,): pass", "(def f (a /) [(pass)])"},
		{"def posonly defaults", "def f(a, b=1, /, c=2): pass", "(def f (a b=1 / c=2) [(pass)])"},
		{"def star args", "def f(a, *args): pass", "(def f (a *args) [(pass)])"},
		{"def star args only", "def f(*args): pass", "(def f (*args) [(pass)])"},
		{"def star args trailing comma", "def f(*args,): pass", "(def f (*args) [(pass)])"},
		{"def star then kwonly", "def f(*args, b, c=1): pass", "(def f (*args b c=1) [(pass)])"},
		{"def bare star", "def f(a, *, b): pass", "(def f (a * b) [(pass)])"},
		{"def bare star only kwonly", "def f(*, a): pass", "(def f (* a) [(pass)])"},
		{"def kwonly default then plain", "def f(a, *, b=1, c): pass", "(def f (a * b=1 c) [(pass)])"},
		{"def kwargs", "def f(**kw): pass", "(def f (**kw) [(pass)])"},
		{"def kwargs trailing comma", "def f(a, **kw,): pass", "(def f (a **kw) [(pass)])"},
		{"def default before star args", "def f(a=1, *b): pass", "(def f (a=1 *b) [(pass)])"},
		{"def default before kwargs", "def f(a=1, **kw): pass", "(def f (a=1 **kw) [(pass)])"},
		{"def every kind", "def f(a, b=1, /, c=2, *args, e, g=3, **kw): pass",
			"(def f (a b=1 / c=2 *args e g=3 **kw) [(pass)])"},
		{"def posonly slash then bare star", "def f(a, /, b, *, c): pass",
			"(def f (a / b * c) [(pass)])"},
		{"call keyword", "f(a=1)", "(expr (call f a=1))"},
		{"call keyword trailing comma", "f(a=1,)", "(expr (call f a=1))"},
		{"call keywords after positionals", "f(1, 2, b=3, c=4)", "(expr (call f 1 2 b=3 c=4))"},
		{"call keyword complex value", "f(a=1 + 2, b=g(c))", "(expr (call f a=(+ 1 2) b=(call g c)))"},
		{"call keyword eqeq is comparison", "f(a == 1)", "(expr (call f (cmp a == 1)))"},
		{"call walrus stays positional", "f(a := 1)", "(expr (call f (:= a 1)))"},
		{"call fstring keyword arg", `f"{g(a=1)}"`, "(expr (fstr (interp (call g a=1))))"},
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
		{"try bare except", "try:\n    x = 1\nexcept:\n    pass\n",
			"(try [(= x 1)] (except [(pass)]) [] [])"},
		{"try except type", "try:\n    pass\nexcept E:\n    pass\n",
			"(try [(pass)] (except E [(pass)]) [] [])"},
		{"try except as", "try:\n    pass\nexcept E as e:\n    x = e\n",
			"(try [(pass)] (except E as e [(= x e)]) [] [])"},
		{"try except paren tuple", "try:\n    pass\nexcept (A, B):\n    pass\n",
			"(try [(pass)] (except (tuple A B) [(pass)]) [] [])"},
		{"try except paren tuple as", "try:\n    pass\nexcept (A, B) as e:\n    pass\n",
			"(try [(pass)] (except (tuple A B) as e [(pass)]) [] [])"},
		{"try except paren one tuple as", "try:\n    pass\nexcept (A,) as e:\n    pass\n",
			"(try [(pass)] (except (tuple A) as e [(pass)]) [] [])"},
		{"pep758 two types", "try:\n    pass\nexcept A, B:\n    pass\n",
			"(try [(pass)] (except (tuple A B) [(pass)]) [] [])"},
		{"pep758 three types", "try:\n    pass\nexcept A, B, C:\n    pass\n",
			"(try [(pass)] (except (tuple A B C) [(pass)]) [] [])"},
		{"pep758 trailing comma", "try:\n    pass\nexcept A,:\n    pass\n",
			"(try [(pass)] (except (tuple A) [(pass)]) [] [])"},
		{"pep758 tuple element", "try:\n    pass\nexcept (A, B), C:\n    pass\n",
			"(try [(pass)] (except (tuple (tuple A B) C) [(pass)]) [] [])"},
		{"try multiple handlers bare last", "try:\n    pass\nexcept A:\n    pass\nexcept B as b:\n    pass\nexcept:\n    pass\n",
			"(try [(pass)] (except A [(pass)]) (except B as b [(pass)]) (except [(pass)]) [] [])"},
		{"try except else", "try:\n    pass\nexcept E:\n    pass\nelse:\n    x = 1\n",
			"(try [(pass)] (except E [(pass)]) [(= x 1)] [])"},
		{"try except finally", "try:\n    pass\nexcept E:\n    pass\nfinally:\n    x = 1\n",
			"(try [(pass)] (except E [(pass)]) [] [(= x 1)])"},
		{"try except else finally", "try:\n    pass\nexcept E:\n    pass\nelse:\n    x = 1\nfinally:\n    y = 2\n",
			"(try [(pass)] (except E [(pass)]) [(= x 1)] [(= y 2)])"},
		{"try finally only", "try:\n    pass\nfinally:\n    pass\n",
			"(try [(pass)] [] [(pass)])"},
		{"try one line suites", "try: pass\nexcept: pass\n",
			"(try [(pass)] (except [(pass)]) [] [])"},
		{"except attribute matcher", "try:\n    pass\nexcept a.b.C as e:\n    pass\n",
			"(try [(pass)] (except (. (. a b) C) as e [(pass)]) [] [])"},
		{"except call matcher", "try:\n    pass\nexcept f():\n    pass\n",
			"(try [(pass)] (except (call f) [(pass)]) [] [])"},
		{"nested try", "try:\n    try:\n        pass\n    finally:\n        pass\nexcept E:\n    pass\n",
			"(try [(try [(pass)] [] [(pass)])] (except E [(pass)]) [] [])"},
		{"raise bare", "raise", "(raise)"},
		{"raise name", "raise ValueError", "(raise ValueError)"},
		{"raise call", "raise ValueError('bad')", `(raise (call ValueError "bad"))`},
		{"raise from", "raise E from cause", "(raise E from cause)"},
		{"raise from None", "raise E from None", "(raise E from None)"},
		{"raise paren tuple", "raise (A, B)", "(raise (tuple A B))"},
		{"raise in def", "def f():\n    raise StopIteration\n", "(def f () [(raise StopIteration)])"},
		{"raise with semicolon", "raise; x = 1", "(raise) (= x 1)"},
		{"assert test", "assert x", "(assert x)"},
		{"assert msg", "assert x, 'bad'", `(assert x "bad")`},
		{"assert comparison", "assert x == 1, y", "(assert (cmp x == 1) y)"},
		{"assert paren tuple test", "assert (x, 'm')", `(assert (tuple x "m"))`},
		{"assert in suite", "if x:\n    assert y, 'no'\n", `(if x [(assert y "no")] [])`},
		{"del name", "del x", "(del x)"},
		{"del multiple", "del a, b[0], c.d", "(del a ([] b 0) (. c d))"},
		{"del paren tuple", "del (a, b)", "(del a b)"},
		{"del nested paren tuple", "del (a, (b, c))", "(del a b c)"},
		{"del trailing comma", "del x,", "(del x)"},
		{"del slice", "del x[1:3]", "(del ([] x (slice 1 3 _)))"},
		{"del empty tuple", "del ()", "(del)"},
		{"walrus parenthesized", "(x := 1)", "(expr (:= x 1))"},
		{"walrus if cond", "if x := f(): pass", "(if (:= x (call f)) [(pass)] [])"},
		{"walrus elif cond", "if a: pass\nelif y := g(): pass\n",
			"(if a [(pass)] [(if (:= y (call g)) [(pass)] [])])"},
		{"walrus while cond", "while chunk := read(): pass",
			"(while (:= chunk (call read)) [(pass)] [])"},
		{"walrus call arg", "f(x := 1, 2)", "(expr (call f (:= x 1) 2))"},
		{"walrus nested value", "(x := (y := 1))", "(expr (:= x (:= y 1)))"},
		{"walrus in paren tuple", "(a, b := 2)", "(expr (tuple a (:= b 2)))"},
		{"walrus ifexp value", "(x := 1 if c else 2)", "(expr (:= x (ifexp c 1 2)))"},
		{"ifexp", "x = 1 if c else 2", "(= x (ifexp c 1 2))"},
		{"ifexp right assoc", "a if c1 else b if c2 else c",
			"(expr (ifexp c1 a (ifexp c2 b c)))"},
		{"ifexp binds looser than or", "a or b if c else d", "(expr (ifexp c (or a b) d))"},
		{"ifexp in call arg", "f(a if c else b)", "(expr (call f (ifexp c a b)))"},
		{"ifexp in return", "return a if c else b", "(return (ifexp c a b))"},
		{"slice lo hi", "x[a:b]", "(expr ([] x (slice a b _)))"},
		{"slice lo hi step", "x[a:b:c]", "(expr ([] x (slice a b c)))"},
		{"slice bare", "x[:]", "(expr ([] x (slice _ _ _)))"},
		{"slice step only", "x[::2]", "(expr ([] x (slice _ _ 2)))"},
		{"slice lo only", "x[1:]", "(expr ([] x (slice 1 _ _)))"},
		{"slice hi only", "x[:n]", "(expr ([] x (slice _ n _)))"},
		{"slice negative step", "x[::-1]", "(expr ([] x (slice _ _ (neg 1))))"},
		{"slice target", "x[a:b] = y", "(= ([] x (slice a b _)) y)"},
		{"plain index stays plain", "x[i]", "(expr ([] x i))"},
		{"star middle target", "a, *b, c = d", "(= (tuple a (* b) c) d)"},
		{"star first target", "*a, b = c", "(= (tuple (* a) b) c)"},
		{"star last target", "a, *b = c", "(= (tuple a (* b)) c)"},
		{"star for target", "for a, *b in x: pass", "(for (tuple a (* b)) x [(pass)] [])"},
		{"star for target first", "for *a, b in x: pass", "(for (tuple (* a) b) x [(pass)] [])"},
		{"chained mixed targets", "a, b = c = [1, 2]", "(= (tuple a b) c (list 1 2))"},
		{"fstring plain folds to string", `f"plain"`, `(expr "plain")`},
		{"fstring empty folds", `f""`, `(expr "")`},
		{"fstring doubled braces fold", `f"a{{b}}c"`, `(expr "a{b}c")`},
		{"fstring text and interps", `f"a{x}b{y}"`, `(expr (fstr "a" (interp x) "b" (interp y)))`},
		{"fstring adjacent interps", `f"{x}{y}"`, `(expr (fstr (interp x) (interp y)))`},
		{"fstring conversions", `f"{x!s}{x!r}{x!a}"`, "(expr (fstr (interp x !s) (interp x !r) (interp x !a)))"},
		{"fstring bang eq is comparison", `f"{1!=2}"`, "(expr (fstr (interp (cmp 1 != 2))))"},
		{"fstring spec", `f"{x:>5}"`, `(expr (fstr (interp x :">5")))`},
		{"fstring empty spec", `f"{x:}"`, `(expr (fstr (interp x :"")))`},
		{"fstring spec escape processed", `f"{x:\x3e5}"`, `(expr (fstr (interp x :">5")))`},
		{"fstring colon eq is spec", `f"{x:=5}"`, `(expr (fstr (interp x :"=5")))`},
		{"fstring conv then spec", `f"{x!r:>5}"`, `(expr (fstr (interp x !r :">5")))`},
		{"fstring eq bare", `f"{x=}"`, `(expr (fstr (interp x ="x=")))`},
		{"fstring eq spaces both sides", `f"{x = }"`, `(expr (fstr (interp x ="x = ")))`},
		{"fstring eq space before", `f"{x =}"`, `(expr (fstr (interp x ="x =")))`},
		{"fstring eq space after", `f"{x= }"`, `(expr (fstr (interp x ="x= ")))`},
		{"fstring eq leading space kept", `f"{ x = }"`, `(expr (fstr (interp x =" x = ")))`},
		{"fstring eq then conv and spec", `f"{x=!r:>5}"`, `(expr (fstr (interp x ="x=" !r :">5")))`},
		{"fstring eq expression text", `f"{x+1=}"`, `(expr (fstr (interp (+ x 1) ="x+1=")))`},
		{"fstring eq tuple", `f"{x,1=}"`, `(expr (fstr (interp (tuple x 1) ="x,1=")))`},
		{"fstring eq newline in triple", "f\"\"\"{x =\n}\"\"\"", `(expr (fstr (interp x ="x =\n")))`},
		{"fstring tuple expression", `f"{1, 2}"`, "(expr (fstr (interp (tuple 1 2))))"},
		{"fstring trailing comma tuple", `f"{1,}"`, "(expr (fstr (interp (tuple 1))))"},
		{"fstring walrus in parens", `f"{(y := 5)}"`, "(expr (fstr (interp (:= y 5))))"},
		{"fstring inner string same quote", `f"{"a" + b}"`, `(expr (fstr (interp (+ "a" b))))`},
		{"fstring dict subscript in braces", `f"{ {1: 2}[1] }"`, "(expr (fstr (interp ([] (dict (1 2)) 1))))"},
		{"fstring multiline expression", "f\"{1 +\n2}\"", "(expr (fstr (interp (+ 1 2))))"},
		{"fstring comment in braces", "f\"{1 # c\n}\"", "(expr (fstr (interp 1)))"},
		{"fstring escape in inner string", `f"{'\n'}"`, `(expr (fstr (interp "\n")))`},
		{"fstring backslash before brace", `f"\{x}"`, `(expr (fstr "\\" (interp x)))`},
		{"fstring triple text", "f'''a\nb{x}'''", `(expr (fstr "a\nb" (interp x)))`},
		{"fstring concat plain then f", `"a" f"b{x}"`, `(expr (fstr "ab" (interp x)))`},
		{"fstring concat f then plain", `f"a{x}" "b"`, `(expr (fstr "a" (interp x) "b"))`},
		{"fstring concat two f", `f"a{x}" f"{y}b"`, `(expr (fstr "a" (interp x) (interp y) "b"))`},
		{"fstring concat folds when plain", `f"a" f"b"`, `(expr "ab")`},
		{"fstring concat mixed folds", `"a" f"b" "c"`, `(expr "abc")`},
		{"fstring as assign value", `s = f"n={n}"`, `(= s (fstr "n=" (interp n)))`},
		{"fstring in call", `log(f"{x}", 1)`, "(expr (call log (fstr (interp x)) 1))"},
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
		{"with", "with open(f) as g: pass", "with statements are not supported yet"},
		{"global", "global x", "global statements are not supported yet"},
		{"nonlocal", "nonlocal x", "nonlocal statements are not supported yet"},
		{"async def", "async def f(): pass", "async is not supported yet"},
		{"lambda", "x = lambda a: a", "lambda expressions are not supported yet"},
		{"yield", "yield 1", "yield expressions are not supported yet"},
		{"await", "x = await f()", "await is not supported yet"},
		{"list comprehension", "[x for x in y]", "list comprehensions are not supported yet"},
		{"dict comprehension", "{k: v for k in y}", "dict comprehensions are not supported yet"},
		{"set comprehension", "{x for x in y}", "set comprehensions are not supported yet"},
		{"generator expr", "(x for x in y)", "generator expressions are not supported yet"},
		{"generator arg", "f(x for x in y)", "generator expressions are not supported yet"},
		{"dict unpacking", "{**a}", "dict unpacking is not supported yet"},
		{"dict then set element", "{1: 2, 3}", "':' expected after dictionary key"},
		{"set then dict entry", "{1, 2: 3}", "invalid syntax"},
		{"set star element", "{*x}", "starred expressions are not supported yet"},
		{"set star later element", "{1, *x}", "starred expressions are not supported yet"},
		{"set double star element", "{1, **x}", "invalid syntax"},
		{"set bare walrus", "{x := 1}", "expected '}'"},
		{"set missing comma", "{1 2}", "expected '}'"},
		{"assign to set", "{1} = x", "cannot assign to set display"},
		{"del set", "del {1}", "cannot delete set display"},
		{"aug set target", "{1} += 1", "'set display' is an illegal expression for augmented assignment"},
		{"walrus set target", "({1} := 2)", "cannot use assignment expressions with set display"},
		{"except as set", "try:\n    pass\nexcept E as {1}:\n    pass\n", "cannot use except statement with set display"},
		{"slice tuple after slice", "x[a:b, c]", "tuples of slices are not supported yet"},
		{"slice tuple before slice", "x[a, b:c]", "tuples of slices are not supported yet"},
		{"star arg", "f(*a)", "'*' argument unpacking is not supported yet"},
		{"double star arg", "f(**a)", "'**' argument unpacking is not supported yet"},
		{"positional after keyword", "f(a=1, 2)", "positional argument follows keyword argument"},
		{"positional between keywords", "f(a, b=1, c)", "positional argument follows keyword argument"},
		{"keyword repeated", "f(b=1, b=2)", "keyword argument repeated: b"},
		{"keyword repeated thrice", "f(b=1, b=2, b=3)", "keyword argument repeated: b"},
		{"paren name keyword", "f((a)=1)", `expression cannot contain assignment, perhaps you meant "=="?`},
		{"literal keyword", "f(1=2)", `expression cannot contain assignment, perhaps you meant "=="?`},
		{"attribute keyword", "f(a.b=1)", `expression cannot contain assignment, perhaps you meant "=="?`},
		{"param annotation", "def f(a: int): pass", "parameter annotations are not supported yet"},
		{"star param annotation", "def f(*args: int): pass", "parameter annotations are not supported yet"},
		{"kwargs annotation", "def f(**kw: int): pass", "parameter annotations are not supported yet"},
		{"annotated default", "def f(a: int = 1): pass", "parameter annotations are not supported yet"},
		{"duplicate param", "def f(a, a): pass", "duplicate argument 'a' in function definition"},
		{"duplicate posonly param", "def f(a, /, a): pass", "duplicate argument 'a' in function definition"},
		{"duplicate star param", "def f(*a, a): pass", "duplicate argument 'a' in function definition"},
		{"duplicate kwargs param", "def f(a, **a): pass", "duplicate argument 'a' in function definition"},
		{"bad param", "def f(1): pass", "expected parameter name"},
		{"non-default after default", "def f(a=1, b): pass", "parameter without a default follows parameter with a default"},
		{"non-default after default across slash", "def f(a=1, /, b): pass", "parameter without a default follows parameter with a default"},
		{"non-default after default posonly", "def f(a, b=1, /, c): pass", "parameter without a default follows parameter with a default"},
		{"default order beats duplicate", "def f(a=1, a): pass", "parameter without a default follows parameter with a default"},
		{"lone slash", "def f(/): pass", "invalid syntax"},
		{"slash first", "def f(/, a): pass", "at least one argument must precede /"},
		{"slash twice", "def f(a, /, b, /): pass", "/ may appear only once"},
		{"slash right after slash", "def f(a, /, /): pass", "/ may appear only once"},
		{"slash after star args", "def f(*a, /): pass", "/ must be ahead of *"},
		{"slash after star args and param", "def f(*args, /, b): pass", "/ must be ahead of *"},
		{"slash after bare star", "def f(*, /): pass", "/ must be ahead of *"},
		{"star twice", "def f(*a, *b): pass", "* argument may appear only once"},
		{"star after bare star", "def f(*, *a): pass", "* argument may appear only once"},
		{"bare star after star args", "def f(*args, *, b): pass", "* argument may appear only once"},
		{"param after kwargs", "def f(**k, a): pass", "arguments cannot follow var-keyword argument"},
		{"kwargs twice", "def f(**k, **j): pass", "arguments cannot follow var-keyword argument"},
		{"slash after kwargs", "def f(**k, /): pass", "arguments cannot follow var-keyword argument"},
		{"star after kwargs", "def f(a, **k, *b): pass", "arguments cannot follow var-keyword argument"},
		{"bare star alone", "def f(*): pass", "named arguments must follow bare *"},
		{"bare star trailing comma", "def f(*,): pass", "named arguments must follow bare *"},
		{"bare star at end", "def f(a, *): pass", "named arguments must follow bare *"},
		{"bare star then kwargs", "def f(*, **k): pass", "named arguments must follow bare *"},
		{"bare star then kwargs after param", "def f(a, *, **k): pass", "named arguments must follow bare *"},
		{"star args default", "def f(*args=1): pass", "var-positional argument cannot have default value"},
		{"kwargs default", "def f(**k=1): pass", "var-keyword argument cannot have default value"},
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
		{"bare star target", "*a = b", "starred assignment target must be in a list or tuple"},
		{"two stars in target", "a, *b, *c = x", "multiple starred expressions in assignment"},
		{"two stars only", "*a, *b = c", "multiple starred expressions in assignment"},
		{"star value", "y = *x", "can't use starred expression here"},
		{"bare star statement", "*a", "can't use starred expression here"},
		{"star in value tuple", "x = *a, b", "iterable unpacking is not supported yet"},
		{"star in paren tuple", "(a, *b) = c", "starred expressions are not supported yet"},
		{"aug tuple target", "a, b += 1", "'tuple' is an illegal expression for augmented assignment"},
		{"aug literal target", "1 += 1", "illegal expression for augmented assignment"},
		{"aug star target", "*a += 1", "'starred' is an illegal expression for augmented assignment"},
		{"for subscript target", "for a[0] in x: pass", "for loop target must be a name or tuple of names"},
		{"for literal target", "for 1 in x: pass", "for loop target must be a name or tuple of names"},
		{"bare star for target", "for *a in b: pass", "starred assignment target must be in a list or tuple"},
		{"two stars in for target", "for a, *b, *c in d: pass", "multiple starred expressions in assignment"},
		{"missing block", "if x:\npass", "expected an indented block"},
		{"unexpected indent first line", "  x = 1", "unexpected indent"},
		{"unexpected indent later", "x = 1\n    y = 2\n", "unexpected indent"},
		{"two exprs on a line", "x 1", "invalid syntax"},
		{"dangling operator", "x = 1 +", "invalid syntax"},
		{"missing colon", "if x\n    pass\n", "expected ':'"},
		{"lexer error surfaces", "x = 0x", "invalid hexadecimal literal"},
		{"try alone", "try:\n    pass\nx = 1\n", "expected 'except' or 'finally' block"},
		{"try at eof", "try:\n    pass\n", "expected 'except' or 'finally' block"},
		{"try else only", "try:\n    pass\nelse:\n    pass\n", "expected 'except' or 'finally' block"},
		{"try else finally", "try:\n    pass\nelse:\n    pass\nfinally:\n    pass\n", "expected 'except' or 'finally' block"},
		{"else before except", "try:\n    pass\nelse:\n    pass\nexcept E:\n    pass\n", "expected 'except' or 'finally' block"},
		{"bare except not last", "try:\n    pass\nexcept:\n    pass\nexcept E:\n    pass\n", "default 'except:' must be last"},
		{"two bare excepts", "try:\n    pass\nexcept:\n    pass\nexcept:\n    pass\n", "default 'except:' must be last"},
		{"pep758 with as", "try:\n    pass\nexcept A, B as e:\n    pass\n", "multiple exception types must be parenthesized when using 'as'"},
		{"except star", "try:\n    pass\nexcept* E:\n    pass\n", "except* is not supported yet"},
		{"except star spaced", "try:\n    pass\nexcept *E:\n    pass\n", "except* is not supported yet"},
		{"except as attribute", "try:\n    pass\nexcept E as a.b:\n    pass\n", "cannot use except statement with attribute"},
		{"except as subscript", "try:\n    pass\nexcept E as a[0]:\n    pass\n", "cannot use except statement with subscript"},
		{"except as call", "try:\n    pass\nexcept E as f():\n    pass\n", "cannot use except statement with function call"},
		{"except as tuple", "try:\n    pass\nexcept E as (a, b):\n    pass\n", "cannot use except statement with tuple"},
		{"except as list", "try:\n    pass\nexcept E as [a]:\n    pass\n", "cannot use except statement with list"},
		{"except as dict", "try:\n    pass\nexcept E as {}:\n    pass\n", "cannot use except statement with dict literal"},
		{"except as int", "try:\n    pass\nexcept E as 1:\n    pass\n", "cannot use except statement with literal"},
		{"except as string", "try:\n    pass\nexcept E as 's':\n    pass\n", "cannot use except statement with literal"},
		{"except as True", "try:\n    pass\nexcept E as True:\n    pass\n", "cannot use except statement with True"},
		{"except as None", "try:\n    pass\nexcept E as None:\n    pass\n", "cannot use except statement with None"},
		{"except as expression", "try:\n    pass\nexcept E as a + b:\n    pass\n", "cannot use except statement with expression"},
		{"except as paren name", "try:\n    pass\nexcept E as (a):\n    pass\n", "cannot use except statement with name"},
		{"except as then comma", "try:\n    pass\nexcept A as e, B:\n    pass\n", "invalid syntax"},
		{"except comma then as", "try:\n    pass\nexcept A, as e:\n    pass\n", "invalid syntax"},
		{"except missing colon", "try:\n    pass\nexcept E\n    pass\n", "expected ':'"},
		{"try in simple line", "x = 1; try: pass", "invalid syntax"},
		{"stray finally after try", "try:\n    pass\nfinally:\n    pass\nfinally:\n    pass\n", "unexpected keyword 'finally'"},
		{"stray except after finally", "try:\n    pass\nexcept:\n    pass\nfinally:\n    pass\nexcept E:\n    pass\n", "unexpected keyword 'except'"},
		{"raise from missing exc", "raise from x", "invalid syntax"},
		{"raise bare tuple", "raise A, B", "invalid syntax"},
		{"raise trailing comma", "raise E,", "invalid syntax"},
		{"raise from dangling", "raise E from", "invalid syntax"},
		{"raise double from", "raise A from B from C", "invalid syntax"},
		{"assert bare", "assert", "invalid syntax"},
		{"assert three parts", "assert x, y, z", "invalid syntax"},
		{"assert msg trailing comma", "assert x, y,", "invalid syntax"},
		{"del literal", "del 1", "cannot delete literal"},
		{"del string", "del 's'", "cannot delete literal"},
		{"del call", "del f()", "cannot delete function call"},
		{"del expression", "del x + y", "cannot delete expression"},
		{"del paren expression", "del (a + b)", "cannot delete expression"},
		{"del True", "del True", "cannot delete True"},
		{"del False", "del False", "cannot delete False"},
		{"del None", "del None", "cannot delete None"},
		{"del dict", "del {}", "cannot delete dict literal"},
		{"del starred", "del *a", "cannot delete starred"},
		{"del starred in list", "del *a, b", "cannot delete starred"},
		{"del list target", "del [a]", "list deletion targets are not supported yet"},
		{"del bare", "del", "invalid syntax"},
		{"bare walrus statement", "x := 1", "invalid syntax"},
		{"walrus attribute target", "(x.y := 1)", "cannot use assignment expressions with attribute"},
		{"walrus subscript target", "(x[0] := 1)", "cannot use assignment expressions with subscript"},
		{"walrus call target", "(f() := 1)", "cannot use assignment expressions with function call"},
		{"walrus tuple target", "((a, b) := 1)", "cannot use assignment expressions with tuple"},
		{"walrus paren name target", "((x) := 1)", "cannot use assignment expressions with name"},
		{"walrus literal target", "(1 := 2)", "cannot use assignment expressions with literal"},
		{"walrus string target", "('s' := 1)", "cannot use assignment expressions with literal"},
		{"walrus True target", "(True := 1)", "cannot use assignment expressions with True"},
		{"walrus False target", "(False := 1)", "cannot use assignment expressions with False"},
		{"walrus None target", "(None := 1)", "cannot use assignment expressions with None"},
		{"walrus list target", "([a] := 1)", "cannot use assignment expressions with list"},
		{"walrus dict target", "({} := 1)", "cannot use assignment expressions with dict literal"},
		{"walrus expression target", "(a + b := 1)", "cannot use assignment expressions with expression"},
		{"walrus chained", "(x := y := 1)", "invalid syntax"},
		{"ifexp missing else", "x = 1 if y", "expected 'else' after 'if' expression"},
		{"ifexp missing else in call", "f(a if b)", "expected 'else' after 'if' expression"},
		{"fstring junk after expression", `f"{x;}"`, "f-string: expecting '=', or '!', or ':', or '}'"},
		{"fstring dangling operator", `f"{1+}"`, "f-string: expecting '=', or '!', or ':', or '}'"},
		{"fstring dangling comparison", `f"{x==}"`, "f-string: expecting '=', or '!', or ':', or '}'"},
		{"fstring two expressions", `f"{1 x}"`, "f-string: expecting '=', or '!', or ':', or '}'"},
		{"fstring bare star", `f"{*x}"`, "can't use starred expression here"},
		{"fstring star in tuple", `f"{*a, b}"`, "iterable unpacking is not supported yet"},
		{"fstring repeated keyword inside", `f"{g(a=1, a=2)}"`, "keyword argument repeated: a"},
		{"fstring lambda inside", `f"{(lambda: x)}"`, "lambda expressions are not supported yet"},
		{"fstring yield inside", `f"{yield}"`, "yield expressions are not supported yet"},
		{"assign to fstring", `f"{x}" = 1`, "cannot assign to f-string expression"},
		{"assign to folded fstring", `f"a" = 1`, "cannot assign to literal"},
		{"aug assign to fstring", `f"{x}" += 1`, "'f-string expression' is an illegal expression for augmented assignment"},
		{"del fstring", `del f"{x}"`, "cannot delete f-string expression"},
		{"walrus fstring target", `(f"{x}" := 1)`, "cannot use assignment expressions with f-string expression"},
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

func TestTryPositions(t *testing.T) {
	src := "try:\n    pass\nexcept E as e:\n    raise\nfinally:\n    assert x, y\n"
	m, err := Parse([]byte(src), "test.py")
	if err != nil {
		t.Fatal(err)
	}
	tr := m.Body[0].(*Try)
	if tr.Pos_ != (Pos{Line: 1, Col: 1}) {
		t.Errorf("try pos %+v", tr.Pos_)
	}
	h := tr.Handlers[0]
	if h.Pos_ != (Pos{Line: 3, Col: 1}) {
		t.Errorf("except pos %+v", h.Pos_)
	}
	r := h.Body[0].(*Raise)
	if r.Pos_ != (Pos{Line: 4, Col: 5}) {
		t.Errorf("raise pos %+v", r.Pos_)
	}
	if r.Exc != nil || r.Cause != nil {
		t.Errorf("bare raise carries exc %v cause %v", r.Exc, r.Cause)
	}
	a := tr.Final[0].(*Assert)
	if a.Pos_ != (Pos{Line: 6, Col: 5}) {
		t.Errorf("assert pos %+v", a.Pos_)
	}
	if a.Msg == nil {
		t.Error("assert msg missing")
	}
}

func TestRaiseFromNoneShape(t *testing.T) {
	m, err := Parse([]byte("raise E from None"), "test.py")
	if err != nil {
		t.Fatal(err)
	}
	r := m.Body[0].(*Raise)
	if _, ok := r.Cause.(*NoneLit); !ok {
		t.Errorf("cause is %T, want *NoneLit", r.Cause)
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
	if got := call.Args[0].Value.Span(); got != (Pos{Line: 2, Col: 7}) {
		t.Errorf("call arg span %+v", got)
	}
}
