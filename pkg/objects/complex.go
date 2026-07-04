package objects

import (
	"math"
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
	return 0, 0, false
}

// asComplex coerces an operand to complex parts: a complex keeps its parts and
// an int, bool or float becomes a real value with a zero imaginary part. Any
// other type reports ok=false so the operator falls back to its type error.
func asComplex(o Object) (re, im float64, ok bool) {
	if c, isC := o.(*complexObject); isC {
		return c.re, c.im, true
	}
	if f, isF := AsFloat(o); isF {
		return f, 0, true
	}
	return 0, 0, false
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

// complexMethod dispatches the complex methods. Only conjugate exists so far;
// any other name is the standard no-attribute error.
func complexMethod(c *complexObject, name string, args []Object) (Object, error) {
	if name != "conjugate" {
		return nil, noAttr(c, name)
	}
	if len(args) != 0 {
		return nil, Raise(TypeError, "conjugate() takes no arguments (%d given)", len(args))
	}
	return NewComplex(c.re, -c.im), nil
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
			return nil, Raise(TypeError, "complex() argument must be a string or a number, not %s", real.TypeName())
		}
		rr, ri = re, im
		_, realIsC = real.(*complexObject)
	}
	ie, ii := 0.0, 0.0
	imagIsC := false
	if imag != nil {
		e, i, ok := asComplex(imag)
		if !ok {
			return nil, Raise(TypeError, "complex() argument must be a string or a number, not %s", imag.TypeName())
		}
		ie, ii = e, i
		_, imagIsC = imag.(*complexObject)
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
		return math.NaN(), i + 3, true
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
