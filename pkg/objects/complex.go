package objects

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

// complexObject is Python's complex: a pair of float64 parts. Every value is
// boxed, so a complex flows through the same Object interface as int and float
// and the numeric operators in ops.go grow a complex branch rather than a
// separate dispatch. See the numbers-tower probes for the exact repr, hash and
// arithmetic that CPython 3.14 produces.
type complexObject struct{ re, im float64 }

func (*complexObject) TypeName() string { return "complex" }

// NewComplex boxes a real and imaginary part.
func NewComplex(re, im float64) Object { return &complexObject{re: re, im: im} }

// ComplexParts reports the parts of an actual complex, and ok=false for every
// other type; abs() uses it to spot a complex without coercing int or float.
func ComplexParts(o Object) (re, im float64, ok bool) {
	if c, isC := o.(*complexObject); isC {
		return c.re, c.im, true
	}
	// A value subclass of complex reads as the parts its payload holds, the way
	// CPython takes the stored value of a complex subclass directly.
	if p, up := builtinUnwrap(o); up {
		if c, isC := p.(*complexObject); isC {
			return c.re, c.im, true
		}
	}
	return 0, 0, false
}

// asComplex coerces an operand to complex parts: a complex keeps its parts and
// an int, bool or float becomes a real value with a zero imaginary part. Any
// other type reports ok=false so the operator falls back to its type error.
func asComplex(o Object) (re, im float64, ok bool) {
	if c, isC := o.(*complexObject); isC {
		return c.re, c.im, true
	}
	// A value subclass of complex keeps the parts its payload holds.
	if re, im, ok := ComplexParts(o); ok {
		return re, im, true
	}
	if f, isF := AsFloat(o); isF {
		return f, 0, true
	}
	return 0, 0, false
}

// AsComplexParts coerces a value to complex parts the way struct's 'F' and 'D'
// codes take their argument: a complex keeps its parts and an int, bool or
// float becomes a real value with a zero imaginary part. Any other type reports
// ok=false so the caller raises "required argument is not a complex".
func AsComplexParts(o Object) (re, im float64, ok bool) {
	return asComplex(o)
}

// eitherComplex reports whether a or b is an actual complex, the guard the
// operators use before trying the complex coercion.
func eitherComplex(a, b Object) bool {
	if _, isA := a.(*complexObject); isA {
		return true
	}
	_, isB := b.(*complexObject)
	return isB
}

// complexRepr renders a complex the way CPython does: a bare imaginary form
// when the real part is a positive zero, and the parenthesized "(re+imj)" form
// otherwise. The join sign is dropped when the imaginary string already carries
// a minus. Probed on 3.14: repr(complex(0,-0.0)) is '-0j' and
// repr(complex(-0.0,-0.0)) is '(-0-0j)'.
func complexRepr(re, im float64) string {
	if re == 0 && !math.Signbit(re) {
		return complexPart(im) + "j"
	}
	imStr := complexPart(im)
	sign := "+"
	if strings.HasPrefix(imStr, "-") {
		sign = ""
	}
	return "(" + complexPart(re) + sign + imStr + "j)"
}

// complexPart formats one component: float repr with a trailing ".0" trimmed,
// so complex(1,0) reprs as "(1+0j)" rather than "(1.0+0.0j)".
func complexPart(f float64) string {
	return strings.TrimSuffix(floatRepr(f), ".0")
}

// complexArith computes a complex +, -, * or /, coercing both operands. It
// reports ok=false when an operand is not numeric so the caller raises the
// unsupported-operand type error, and a non-nil error for division by zero.
func complexArith(op byte, a, b Object) (Object, bool, error) {
	ar, ai, ok1 := asComplex(a)
	br, bi, ok2 := asComplex(b)
	if !ok1 || !ok2 {
		return nil, false, nil
	}
	switch op {
	case '+':
		return NewComplex(ar+br, ai+bi), true, nil
	case '-':
		return NewComplex(ar-br, ai-bi), true, nil
	case '*':
		return NewComplex(ar*br-ai*bi, ar*bi+ai*br), true, nil
	case '/':
		re, im, err := complexQuot(ar, ai, br, bi)
		if err != nil {
			return nil, true, err
		}
		return NewComplex(re, im), true, nil
	}
	return nil, false, nil
}

// complexArithOp is complexArith guarded by the subclass-override check, the
// complex counterpart of floatArith. When either operand is a complex subclass
// instance that overrides the operator's forward or reflected slot, it declines
// so the caller falls through to the dunder protocol rather than coercing the
// payload and computing natively, which would drop the subclass's method and its
// return type. A plain complex, or a subclass that inherits the base arithmetic,
// keeps the native fast path.
func complexArithOp(op byte, a, b Object, forward, reflected string) (Object, bool, error) {
	if numericInstOverrides(a, forward, reflected) || numericInstOverrides(b, forward, reflected) {
		return nil, false, nil
	}
	return complexArith(op, a, b)
}

// complexQuot divides (ar+ai j) by (br+bi j) with Smith's method, scaling by
// the smaller denominator part to limit overflow the way CPython's
// _Py_c_quot does. A zero divisor raises the probed "division by zero".
func complexQuot(ar, ai, br, bi float64) (float64, float64, error) {
	if math.Abs(br) >= math.Abs(bi) {
		if br == 0 && bi == 0 {
			return 0, 0, Raise(ZeroDivisionError, "division by zero")
		}
		ratio := bi / br
		denom := br + bi*ratio
		return (ar + ai*ratio) / denom, (ai - ar*ratio) / denom, nil
	}
	ratio := br / bi
	denom := br*ratio + bi
	return (ar*ratio + ai) / denom, (ai*ratio - ar) / denom, nil
}

// complexPow raises (ar+ai j) to (br+bi j). A real integer exponent with a
// small magnitude uses repeated squaring for an exact result, matching
// CPython's c_powi fast path; everything else takes the general polar form.
// A zero base with a negative or complex exponent raises the probed error.
func complexPow(ar, ai, br, bi float64) (Object, error) {
	if bi == 0 && br == math.Trunc(br) && math.Abs(br) <= 100 {
		n := int(br)
		if ar == 0 && ai == 0 {
			if n == 0 {
				return NewComplex(1, 0), nil
			}
			if n < 0 {
				return nil, Raise(ZeroDivisionError, "zero to a negative or complex power")
			}
			return NewComplex(0, 0), nil
		}
		return complexPowi(ar, ai, n), nil
	}
	if ar == 0 && ai == 0 {
		return nil, Raise(ZeroDivisionError, "zero to a negative or complex power")
	}
	return cPow(ar, ai, br, bi), nil
}

// complexPowi computes a complex to a small integer power by squaring, then
// inverts for a negative exponent. The base is non-zero here.
func complexPowi(ar, ai float64, n int) Object {
	neg := n < 0
	if neg {
		n = -n
	}
	rr, ri := 1.0, 0.0
	pr, pi := ar, ai
	for n > 0 {
		if n&1 == 1 {
			rr, ri = rr*pr-ri*pi, rr*pi+ri*pr
		}
		n >>= 1
		if n > 0 {
			pr, pi = pr*pr-pi*pi, 2*pr*pi
		}
	}
	if neg {
		re, im, _ := complexQuot(1, 0, rr, ri)
		return NewComplex(re, im)
	}
	return NewComplex(rr, ri)
}

// cPow is the general complex power in polar form, following CPython's _Py_c_pow:
// magnitude and phase of the base drive an exp/atan2 evaluation.
func cPow(ar, ai, br, bi float64) Object {
	vabs := math.Hypot(ar, ai)
	length := math.Pow(vabs, br)
	at := math.Atan2(ai, ar)
	phase := at * br
	if bi != 0 {
		length /= math.Exp(at * bi)
		phase += bi * math.Log(vabs)
	}
	return NewComplex(length*math.Cos(phase), length*math.Sin(phase))
}

// ComplexAbs is the magnitude hypot(re, im), raising OverflowError when a finite
// pair produces an infinite result, the way CPython's _Py_c_abs signals ERANGE.
// An infinite part yields inf without error and a nan part yields nan, so abs()
// and complex.__abs__ agree on the whole domain.
//
// CPython's _Py_c_abs calls the platform C hypot, which is correctly rounded on
// glibc and macOS, so the result is the nearest double to the true magnitude.
// Go's math.Hypot is not correctly rounded and lands a unit in the last place off
// on around a quarter of small integer pairs (hypot(2, 3) is the first), so the
// finite case here rounds the true magnitude once through extended precision to
// match CPython byte for byte. The non-finite cases follow C99's hypot rules
// (an infinite part wins over a nan, otherwise a nan yields nan), which Go's
// math.Hypot already implements, so they defer to it.
func ComplexAbs(re, im float64) (float64, error) {
	if math.IsInf(re, 0) || math.IsInf(im, 0) || math.IsNaN(re) || math.IsNaN(im) {
		return math.Hypot(re, im), nil
	}
	r := CorrectlyRoundedHypot(re, im)
	if math.IsInf(r, 0) {
		return 0, Raise(OverflowError, "absolute value too large")
	}
	return r, nil
}

// CorrectlyRoundedHypot returns sqrt(sum of the squares of xs) correctly rounded
// to double, for finite coordinates. Each square is formed in a big.Float wide
// enough to hold it exactly (a double squared needs 106 bits) and the running sum
// carries far more precision than the final double needs, then the square root is
// taken and rounded once to double, so the result is the nearest double to the
// exact magnitude, the value the platform C hypot and CPython's math.hypot both
// yield. big.Float carries its own exponent, so squares that overflow or underflow
// double (1e200 or 1e-200) still contribute, and only a true magnitude past the
// double range rounds to inf. Callers screen out infinite and nan coordinates
// first, so this only sees finite input.
func CorrectlyRoundedHypot(xs ...float64) float64 {
	const prec = 500
	sum := new(big.Float).SetPrec(prec)
	for _, v := range xs {
		t := new(big.Float).SetPrec(prec).SetFloat64(v)
		t.Mul(t, t)
		sum.Add(sum, t)
	}
	if sum.Sign() == 0 {
		return 0
	}
	sum.Sqrt(sum)
	r, _ := sum.Float64()
	return r
}

// complexMethodNames is the method and operator-dunder surface a complex answers,
// so a bound read (c.__add__) and a direct call (c.__add__(x)) agree on what a
// complex exposes and hasattr matches CPython. complex carries its arithmetic
// dunders the same additive way int does: the operators still route through Add
// and friends, this only makes the slots readable as callables.
var complexMethodNames = map[string]bool{
	"conjugate": true,
	"__add__":   true, "__radd__": true,
	"__sub__": true, "__rsub__": true,
	"__mul__": true, "__rmul__": true,
	"__truediv__": true, "__rtruediv__": true,
	"__pow__": true, "__rpow__": true,
	"__neg__": true, "__pos__": true, "__abs__": true,
	"__eq__": true, "__ne__": true,
	"__bool__": true, "__hash__": true,
	"__repr__": true, "__str__": true, "__format__": true,
	"__getnewargs__": true, "__complex__": true,
}

// complexMethod dispatches a complex method or operator dunder. The arithmetic
// dunders decline a non-numeric operand with NotImplemented rather than raising,
// exactly like int's slots, so a mixed pair hands off to the other operand.
func complexMethod(c *complexObject, name string, args []Object) (Object, error) {
	switch name {
	case "conjugate":
		if len(args) != 0 {
			return nil, Raise(TypeError, "conjugate() takes no arguments (%d given)", len(args))
		}
		return NewComplex(c.re, -c.im), nil
	case "__add__":
		return complexBinDunder(c, '+', args, false)
	case "__radd__":
		return complexBinDunder(c, '+', args, true)
	case "__sub__":
		return complexBinDunder(c, '-', args, false)
	case "__rsub__":
		return complexBinDunder(c, '-', args, true)
	case "__mul__":
		return complexBinDunder(c, '*', args, false)
	case "__rmul__":
		return complexBinDunder(c, '*', args, true)
	case "__truediv__":
		return complexBinDunder(c, '/', args, false)
	case "__rtruediv__":
		return complexBinDunder(c, '/', args, true)
	case "__pow__":
		return complexPowDunder(c, args, false)
	case "__rpow__":
		return complexPowDunder(c, args, true)
	case "__neg__":
		if err := complexNoArgs(args); err != nil {
			return nil, err
		}
		return NewComplex(-c.re, -c.im), nil
	case "__pos__":
		if err := complexNoArgs(args); err != nil {
			return nil, err
		}
		return NewComplex(c.re, c.im), nil
	case "__abs__":
		if err := complexNoArgs(args); err != nil {
			return nil, err
		}
		r, err := ComplexAbs(c.re, c.im)
		if err != nil {
			return nil, err
		}
		return NewFloat(r), nil
	case "__eq__", "__ne__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
		}
		or, oi, ok := asComplex(args[0])
		if !ok {
			return NotImplemented, nil
		}
		eq := c.re == or && c.im == oi
		if name == "__ne__" {
			eq = !eq
		}
		return NewBool(eq), nil
	case "__bool__":
		if err := complexNoArgs(args); err != nil {
			return nil, err
		}
		return NewBool(c.re != 0 || c.im != 0), nil
	case "__hash__":
		if err := complexNoArgs(args); err != nil {
			return nil, err
		}
		h, err := PyHash(c)
		if err != nil {
			return nil, err
		}
		return NewInt(h), nil
	case "__repr__", "__str__":
		if err := complexNoArgs(args); err != nil {
			return nil, err
		}
		return NewStr(complexRepr(c.re, c.im)), nil
	case "__format__":
		if len(args) != 1 {
			return nil, Raise(TypeError, "complex.__format__() takes exactly one argument (%d given)", len(args))
		}
		spec, ok := AsStr(args[0])
		if !ok {
			return nil, Raise(TypeError, "__format__() argument must be str, not %s", args[0].TypeName())
		}
		return Format(c, spec)
	case "__getnewargs__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "complex.__getnewargs__() takes no arguments (%d given)", len(args))
		}
		return NewTuple([]Object{NewFloat(c.re), NewFloat(c.im)}), nil
	case "__complex__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "complex.__complex__() takes no arguments (%d given)", len(args))
		}
		return c, nil
	}
	return nil, noAttr(c, name)
}

// complexNoArgs rejects a positional argument for the argument-free complex
// dunders, matching the C slot wrapper's "expected 0 arguments" wording.
func complexNoArgs(args []Object) error {
	if len(args) != 0 {
		return Raise(TypeError, "expected 0 arguments, got %d", len(args))
	}
	return nil
}

// complexBinDunder computes one of complex's +, -, * or / operator dunders,
// swapping the operands for a reflected slot and declining a non-numeric operand
// with NotImplemented the way int's slots do.
func complexBinDunder(c *complexObject, op byte, args []Object, reflected bool) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
	}
	var self Object = c
	a, b := self, args[0]
	if reflected {
		a, b = args[0], self
	}
	res, ok, err := complexArith(op, a, b)
	if err != nil {
		return nil, err
	}
	if !ok {
		return NotImplemented, nil
	}
	return res, nil
}

// complexPowDunder computes complex's __pow__/__rpow__. The optional second
// argument is the modulo slot ternary pow passes; a complex has no modulo so any
// value but None raises, matching CPython's "complex modulo".
func complexPowDunder(c *complexObject, args []Object, reflected bool) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, Raise(TypeError, "expected 1 or 2 arguments, got %d", len(args))
	}
	if len(args) == 2 && args[1] != None {
		return nil, Raise(ValueError, "complex modulo")
	}
	or, oi, ok := asComplex(args[0])
	if !ok {
		return NotImplemented, nil
	}
	if reflected {
		return complexPow(or, oi, c.re, c.im)
	}
	return complexPow(c.re, c.im, or, oi)
}

// complexLoadAttr reads an attribute off a complex: real and imag answer the
// parts, a method or operator-dunder name binds a callable, and anything else is
// the complex AttributeError.
func complexLoadAttr(c *complexObject, name string) (Object, error) {
	switch name {
	case "real":
		return NewFloat(c.re), nil
	case "imag":
		return NewFloat(c.im), nil
	case "__doc__":
		return None, nil
	}
	if complexMethodNames[name] {
		method := name
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			return complexMethod(c, method, args)
		}), nil
	}
	return nil, Raise(AttributeError, "'complex' object has no attribute '%s'", name)
}

// ComplexFromDunder exposes complexFromDunder for callers such as cmath, which
// coerce an argument through CPython's PyComplex_AsCComplex the same way.
func ComplexFromDunder(o Object) (re, im float64, ok bool, err error) {
	return complexFromDunder(o)
}

// complexFromDunder resolves a user instance to complex components the way
// complex(o) does: __complex__ wins and must return a complex (else
// "__complex__ returned non-complex (type X)"), otherwise __float__ then
// __index__ supply a real value with a zero imaginary part. ok is false when o
// is not a user instance or defines none of the three, leaving the caller's
// "argument must be a string or a number" error.
func complexFromDunder(o Object) (re, im float64, ok bool, err error) {
	x, isInst := o.(*instanceObject)
	if !isInst {
		return 0, 0, false, nil
	}
	if _, has := x.cls.lookup("__complex__"); has {
		r, _, e := instanceSpecial(x, "__complex__")
		if e != nil {
			return 0, 0, true, e
		}
		if !instanceOfBuiltin(r, "complex") {
			return 0, 0, true, Raise(TypeError,
				"__complex__ returned non-complex (type %s)", r.TypeName())
		}
		cr, ci, _ := asComplex(r)
		return cr, ci, true, nil
	}
	// __float__ then __index__, exactly float()'s instance fallback.
	if f, defined, e := FloatFromDunder(o); e != nil {
		return 0, 0, true, e
	} else if defined {
		fv, _ := AsFloat(f)
		return fv, 0, true, nil
	}
	return 0, 0, false, nil
}

// ComplexNew builds a complex from the constructor arguments, either of which
// may be nil when the caller omitted it. A str real parses like a literal and
// forbids a second argument; numeric parts combine as real + imag*1j. The
// error wordings are probed on 3.14.
func ComplexNew(real, imag Object) (Object, error) {
	if real != nil {
		if s, ok := AsStr(real); ok {
			if imag != nil {
				return nil, Raise(TypeError, "complex() argument 'real' must be a real number, not str")
			}
			re, im, ok := ParseComplex(s)
			if !ok {
				return nil, Raise(ValueError, "complex() arg is a malformed string")
			}
			return NewComplex(re, im), nil
		}
	}
	if imag != nil {
		if _, ok := AsStr(imag); ok {
			return nil, Raise(TypeError, "complex() argument 'imag' must be a real number, not str")
		}
	}
	rr, ri := 0.0, 0.0
	realIsC := false
	if real != nil {
		re, im, ok := asComplex(real)
		if !ok {
			// A user numeric supplies the real argument through __complex__, then
			// __float__, then __index__. A resolved value carries its imaginary
			// part, so it routes through the combining path like a complex arg.
			cre, cim, dok, err := complexFromDunder(real)
			if err != nil {
				return nil, err
			}
			if !dok {
				return nil, Raise(TypeError, "complex() argument must be a string or a number, not %s", real.TypeName())
			}
			re, im = cre, cim
			realIsC = true
		}
		rr, ri = re, im
		if !realIsC {
			_, _, realIsC = ComplexParts(real)
		}
	}
	ie, ii := 0.0, 0.0
	imagIsC := false
	if imag != nil {
		e, i, ok := asComplex(imag)
		if !ok {
			return nil, Raise(TypeError, "complex() argument must be a string or a number, not %s", imag.TypeName())
		}
		ie, ii = e, i
		_, _, imagIsC = ComplexParts(imag)
	}
	// With plain real parts CPython sets the components directly, which keeps a
	// signed zero: complex(0, -0.0) is -0j. Only a complex argument takes the
	// real + imag*1j combining path, where (ie+ii j)*j = -ii + ie j.
	if !realIsC && !imagIsC {
		return NewComplex(rr, ie), nil
	}
	return NewComplex(rr-ii, ri+ie), nil
}

// ParseComplex parses complex()'s string form: an optional parenthesized body,
// then a real part, an imaginary part, or "real +/- imagj", with j or J marking
// the imaginary unit. It reports ok=false for any malformed string, which the
// caller turns into the ValueError. Underscores are allowed only between digits.
func ParseComplex(s string) (float64, float64, bool) {
	s = strings.TrimFunc(s, unicode.IsSpace)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimFunc(s[1:len(s)-1], unicode.IsSpace)
	}
	s, ok := stripComplexUnderscores(s)
	if !ok || s == "" {
		return 0, 0, false
	}
	v1, i, has1 := scanComplexFloat(s, 0)
	if i < len(s) && isImagUnit(s[i]) {
		i++
		if i != len(s) {
			return 0, 0, false
		}
		return 0, v1, true
	}
	if !has1 {
		return 0, 0, false
	}
	if i == len(s) {
		return v1, 0, true
	}
	if s[i] != '+' && s[i] != '-' {
		return 0, 0, false
	}
	v2, n2, _ := scanComplexFloat(s, i)
	if n2 == i {
		return 0, 0, false
	}
	i = n2
	if i >= len(s) || !isImagUnit(s[i]) {
		return 0, 0, false
	}
	i++
	if i != len(s) {
		return 0, 0, false
	}
	return v1, v2, true
}

func isImagUnit(c byte) bool { return c == 'j' || c == 'J' }

func isDecDigit(c byte) bool { return c >= '0' && c <= '9' }

// scanComplexFloat reads an optionally signed float starting at i. It returns
// the value, the index just past what it consumed, and whether it saw a real
// number body. When only a sign is present it returns the signed unit 1 so a
// following j reads as "+1j" or "-1j", and sawNumber=false.
func scanComplexFloat(s string, i int) (val float64, next int, sawNumber bool) {
	start := i
	sign := 1.0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	afterSign := i
	lower := strings.ToLower(s[i:])
	switch {
	case strings.HasPrefix(lower, "infinity"):
		return sign * math.Inf(1), i + len("infinity"), true
	case strings.HasPrefix(lower, "inf"):
		return sign * math.Inf(1), i + 3, true
	case strings.HasPrefix(lower, "nan"):
		return FloatNaN(), i + 3, true
	}
	hasDigits := false
	for i < len(s) && isDecDigit(s[i]) {
		i++
		hasDigits = true
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDecDigit(s[i]) {
			i++
			hasDigits = true
		}
	}
	if !hasDigits {
		return sign, afterSign, false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && isDecDigit(s[j]) {
			for j < len(s) && isDecDigit(s[j]) {
				j++
			}
			i = j
		}
	}
	f, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil && !strings.Contains(err.Error(), "range") {
		return sign, afterSign, false
	}
	return f, i, true
}

// stripComplexUnderscores removes digit-group underscores, reporting ok=false
// when an underscore is not flanked by digits, matching Python's numeric rule.
func stripComplexUnderscores(s string) (string, bool) {
	if !strings.Contains(s, "_") {
		return s, true
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			if i == 0 || i == len(s)-1 || !isDecDigit(s[i-1]) || !isDecDigit(s[i+1]) {
				return "", false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String(), true
}
