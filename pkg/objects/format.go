package objects

// Format machinery for f-strings and format(o, spec): the __format__
// dispatch for the boxed types. str, int (and bool through it) and
// float get the full mini-language; every other type keeps
// object.__format__ semantics, str(o) on an empty spec and a TypeError
// otherwise. All output shapes and error texts are probed on CPython
// 3.14, never guessed.

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// fmtSpec is a parsed format mini-language spec:
// [[fill]align][sign][z][#][0][width][,|_][.precision][type].
type fmtSpec struct {
	fill      rune // 0 when unspecified
	align     rune // 0 when unspecified
	sign      rune // 0, '+', '-' or ' '
	coerceNeg bool // 'z' flag, negative zero coercion
	alternate bool // '#' flag
	zeroPad   bool // bare '0' before the width
	width     int
	hasPrec   bool
	prec      int
	grouping  rune // 0, ',' or '_'
	fracGroup rune // 0, ',' or '_': 3.14 fractional-part grouping
	verb      rune // presentation type, 0 when omitted
}

func isFmtAlign(r rune) bool { return r == '<' || r == '>' || r == '^' || r == '=' }

// parseFmtSpec parses a non-empty spec. typeName feeds the "Invalid
// format specifier" text and defaultVerb stands in for an omitted type
// in the grouping compatibility check ('s' for str, 0 for numbers).
// The check order matches CPython's parser: leftover characters win
// over grouping-versus-type complaints, which win over everything the
// per-type formatters validate later.
func parseFmtSpec(spec, typeName string, defaultVerb rune) (fmtSpec, error) {
	rs := []rune(spec)
	sp := fmtSpec{}
	i := 0
	// A two-char fill+align pair wins over a one-char align, so "<<"
	// means fill '<' align '<'.
	switch {
	case len(rs) >= 2 && isFmtAlign(rs[1]):
		sp.fill, sp.align = rs[0], rs[1]
		i = 2
	case len(rs) >= 1 && isFmtAlign(rs[0]):
		sp.align = rs[0]
		i = 1
	}
	if i < len(rs) && (rs[i] == '+' || rs[i] == '-' || rs[i] == ' ') {
		sp.sign = rs[i]
		i++
	}
	if i < len(rs) && rs[i] == 'z' {
		sp.coerceNeg = true
		i++
	}
	if i < len(rs) && rs[i] == '#' {
		sp.alternate = true
		i++
	}
	// A '0' here is the zero-pad shorthand, but only when no explicit
	// fill was given. Probed on 3.14: format(42, '*<06') is '42****',
	// the '0' folds into the width once a fill exists.
	if sp.fill == 0 && i < len(rs) && rs[i] == '0' {
		sp.zeroPad = true
		i++
	}
	for i < len(rs) && rs[i] >= '0' && rs[i] <= '9' {
		sp.width = sp.width*10 + int(rs[i]-'0')
		i++
	}
	if i < len(rs) && rs[i] == ',' {
		sp.grouping = ','
		i++
	}
	if i < len(rs) && rs[i] == '_' {
		if sp.grouping != 0 {
			// Probed on 3.14: format(1234, ',_').
			return sp, Raise(ValueError, "Cannot specify both ',' and '_'.")
		}
		sp.grouping = '_'
		i++
	}
	// Probed on 3.14: format(1234, '_,') is the both-separators error,
	// but format(42, ',,') falls through and reads the second comma as
	// the type, giving "Cannot specify ',' with ','." below.
	if sp.grouping == '_' && i < len(rs) && rs[i] == ',' {
		return sp, Raise(ValueError, "Cannot specify both ',' and '_'.")
	}
	if i < len(rs) && rs[i] == '.' {
		i++
		if i >= len(rs) || rs[i] < '0' || rs[i] > '9' {
			// Probed on 3.14: format(1.5, '.') and format(1.5, '.f').
			return sp, Raise(ValueError, "Format specifier missing precision")
		}
		sp.hasPrec = true
		for i < len(rs) && rs[i] >= '0' && rs[i] <= '9' {
			sp.prec = sp.prec*10 + int(rs[i]-'0')
			i++
		}
		// Python 3.14 grew grouping for the fractional part right after
		// the precision. Probed: format(1234.567891, '.6,f') is
		// '1234.567,891' and str ignores it, format('abc', '.2_') is 'ab'.
		if i < len(rs) && rs[i] == ',' {
			sp.fracGroup = ','
			i++
		}
		if i < len(rs) && rs[i] == '_' {
			if sp.fracGroup != 0 {
				// Probed on 3.14: format(1234.567891, '.6,_f').
				return sp, Raise(ValueError, "Cannot specify both ',' and '_'.")
			}
			sp.fracGroup = '_'
			i++
		}
		if sp.fracGroup == '_' && i < len(rs) && rs[i] == ',' {
			return sp, Raise(ValueError, "Cannot specify both ',' and '_'.")
		}
	}
	switch {
	case i == len(rs):
	case i == len(rs)-1:
		sp.verb = rs[i]
	default:
		// Probed on 3.14: format(3, 'q q') names the whole spec and the
		// object's type.
		return sp, Raise(ValueError, "Invalid format specifier '%s' for object of type '%s'", spec, typeName)
	}
	if sp.grouping != 0 {
		verb := sp.verb
		if verb == 0 {
			verb = defaultVerb
		}
		ok := false
		switch verb {
		case 0, 'd', 'e', 'f', 'g', 'E', 'F', 'G', '%':
			ok = true
		case 'b', 'o', 'x', 'X':
			// PEP 515: underscores group binary, octal and hex too, but
			// commas stay decimal-only. Probed on 3.14: format(255, ',x')
			// raises, format(255, '_x') is 'ff'.
			ok = sp.grouping == '_'
		}
		if !ok {
			// Probed on 3.14: format('abc', ',') says 's', format(42, ',q')
			// says 'q', format(42, ',,') says ','.
			return sp, Raise(ValueError, "Cannot specify '%c' with '%c'.", sp.grouping, verb)
		}
	}
	return sp, nil
}

// Format implements format(o, spec), the __format__ dispatch.
func Format(o Object, spec string) (Object, error) {
	// A user __format__ intercepts every spec, empty included: probed on
	// 3.14, format(Fmt(), '') calls __format__ with '' rather than str(o).
	if inst, ok := o.(*instanceObject); ok {
		res, defined, err := instanceSpecial(inst, "__format__", NewStr(spec))
		if defined {
			if err != nil {
				return nil, err
			}
			if _, isStr := res.(*strObject); !isStr {
				return nil, Raise(TypeError, "__format__ must return a str, not %s", res.TypeName())
			}
			return res, nil
		}
	}
	// An empty spec is str(o) for every type. Probed on 3.14:
	// format(True, '') is 'True' and format(None, '') is 'None'. The
	// strict form keeps the 4300-digit int limit in play.
	if spec == "" {
		s, err := StrE(o)
		if err != nil {
			return nil, err
		}
		return NewStr(s), nil
	}
	switch x := o.(type) {
	case *strObject:
		return formatStr(x.v, spec)
	case *intObject:
		return formatInt(x, "int", spec)
	case *boolObject:
		// bool has no __format__ of its own, so a non-empty spec falls
		// to int's, keeping 'bool' in error texts. Probed on 3.14:
		// format(True, 'd') is '1', format(True, 's') raises with 'bool'.
		var v int64
		if x.v {
			v = 1
		}
		return formatInt(&intObject{v: v}, "bool", spec)
	case *floatObject:
		return formatFloat(x.v, "float", spec)
	}
	// object.__format__: any non-empty spec is a TypeError. Probed on
	// 3.14: format(None, '>5') and format([1, 2], 'd').
	return nil, Raise(TypeError, "unsupported format string passed to %s.__format__", o.TypeName())
}

// formatStr handles the 's' presentation: truncation by precision,
// then fill and alignment. Default alignment is '<' and, unlike the
// numeric paths, the zero-pad shorthand keeps it. Probed on 3.14:
// format('abc', '05') is 'abc00'.
func formatStr(s, spec string) (Object, error) {
	sp, err := parseFmtSpec(spec, "str", 's')
	if err != nil {
		return nil, err
	}
	if sp.verb != 0 && sp.verb != 's' {
		// Probed on 3.14: format('abc', 'd'). The unknown-code check runs
		// before the flag checks, format('abc', '+d') also says 'd'.
		return nil, Raise(ValueError, "Unknown format code '%c' for object of type 'str'", sp.verb)
	}
	// Flag rejections in CPython's order: sign, z, #, then '='. The
	// space sign has its own wording. All probed on 3.14.
	switch sp.sign {
	case ' ':
		return nil, Raise(ValueError, "Space not allowed in string format specifier")
	case '+', '-':
		return nil, Raise(ValueError, "Sign not allowed in string format specifier")
	}
	if sp.coerceNeg {
		return nil, Raise(ValueError, "Negative zero coercion (z) not allowed in string format specifier")
	}
	if sp.alternate {
		return nil, Raise(ValueError, "Alternate form (#) not allowed in string format specifier")
	}
	if sp.align == '=' {
		return nil, Raise(ValueError, "'=' alignment not allowed in string format specifier")
	}
	if sp.hasPrec {
		rs := []rune(s)
		if sp.prec < len(rs) {
			s = string(rs[:sp.prec])
		}
	}
	fill := sp.fill
	if fill == 0 {
		fill = ' '
		if sp.zeroPad {
			fill = '0'
		}
	}
	align := sp.align
	if align == 0 {
		align = '<'
	}
	return NewStr(padText(s, fill, align, sp.width)), nil
}

// padText aligns body within width code points using fill. A centered
// value puts the odd fill char on the right, like CPython.
func padText(body string, fill, align rune, width int) string {
	pad := width - utf8.RuneCountInString(body)
	if pad <= 0 {
		return body
	}
	switch align {
	case '<':
		return body + strings.Repeat(string(fill), pad)
	case '^':
		left := pad / 2
		return strings.Repeat(string(fill), left) + body + strings.Repeat(string(fill), pad-left)
	}
	return strings.Repeat(string(fill), pad) + body
}

// groupDigits inserts sep into a run of digits every size digits from
// the right. When minWidth is positive the run is first zero-extended
// so the grouped result is at least minWidth wide without ever leading
// with a separator, which can overshoot the requested width. Probed on
// 3.14: format(1234, '08,') is '0,001,234', nine characters.
func groupDigits(digits string, size int, sep rune, minWidth int) string {
	n := len(digits)
	for n+(n-1)/size < minWidth {
		n++
	}
	if n > len(digits) {
		digits = strings.Repeat("0", n-len(digits)) + digits
	}
	var b strings.Builder
	first := n % size
	if first == 0 {
		first = size
	}
	b.WriteString(digits[:first])
	for i := first; i < n; i += size {
		b.WriteRune(sep)
		b.WriteString(digits[i : i+size])
	}
	return b.String()
}

// buildNumber assembles sign, prefix, groupable digits and the rest of
// the rendered number with fill and alignment. The numeric default
// alignment is '>', or '=' under the zero-pad shorthand. '=' padding
// goes between the prefix and the digits: probed on 3.14, format(-42,
// '*=#8x') is '-0x***2a'. Zero padding under grouping extends the
// digit run with separators instead, but only when there are digits to
// extend; probed on 3.14, format(float('inf'), '010,.1f') is
// '0000000inf'.
func buildNumber(sp fmtSpec, sign, prefix, digits, rest string, groupSize int) string {
	fill := sp.fill
	if fill == 0 {
		fill = ' '
		if sp.zeroPad {
			fill = '0'
		}
	}
	align := sp.align
	if align == 0 {
		align = '>'
		if sp.zeroPad {
			align = '='
		}
	}
	if sp.grouping != 0 && digits != "" {
		minWidth := 0
		if fill == '0' && align == '=' {
			minWidth = sp.width - utf8.RuneCountInString(sign) - len(prefix) - utf8.RuneCountInString(rest)
		}
		digits = groupDigits(digits, groupSize, sp.grouping, minWidth)
	}
	body := digits + rest
	pad := sp.width - utf8.RuneCountInString(sign) - len(prefix) - utf8.RuneCountInString(body)
	if pad > 0 && align == '=' {
		return sign + prefix + strings.Repeat(string(fill), pad) + body
	}
	return padText(sign+prefix+body, fill, align, sp.width)
}

// groupFraction inserts sep into the fractional digits of a rendered
// float, every three from the left, stopping at the exponent or the
// percent sign. Probed on 3.14: format(1234.56789, '.4,f') is
// '1234.567,9' and format(1234.567891, '.6,e') is '1.234,568e+03'.
func groupFraction(body string, sep rune) string {
	dot := strings.IndexByte(body, '.')
	if dot < 0 {
		return body
	}
	end := dot + 1
	for end < len(body) && body[end] >= '0' && body[end] <= '9' {
		end++
	}
	frac := body[dot+1 : end]
	if len(frac) <= 3 {
		return body
	}
	var b strings.Builder
	b.WriteString(body[:dot+1])
	for i := 0; i < len(frac); i += 3 {
		if i > 0 {
			b.WriteRune(sep)
		}
		j := i + 3
		if j > len(frac) {
			j = len(frac)
		}
		b.WriteString(frac[i:j])
	}
	b.WriteString(body[end:])
	return b.String()
}

// signStr picks the sign text for a number: '-' always shows, and the
// spec's '+' or ' ' applies to non-negatives.
func signStr(neg bool, sign rune) string {
	switch {
	case neg:
		return "-"
	case sign == '+':
		return "+"
	case sign == ' ':
		return " "
	}
	return ""
}

// formatInt handles the integer presentations d b o x X c n and none,
// and hands the float codes off with the value widened. typeName rides
// along so bool errors say 'bool'.
func formatInt(x *intObject, typeName, spec string) (Object, error) {
	sp, err := parseFmtSpec(spec, typeName, 0)
	if err != nil {
		return nil, err
	}
	switch sp.verb {
	case 'e', 'E', 'f', 'F', 'g', 'G', '%':
		// Probed on 3.14: format(42, '.2f') is '42.00' and the float
		// rules apply wholesale, format(42, 'z=+#,.2f') is '+42.00'.
		// Probed: f"{2**11000:.2f}" overflows the widening itself.
		f, _, ferr := asFloatChecked(x)
		if ferr != nil {
			return nil, ferr
		}
		return formatFloatSpec(f, sp)
	case 0, 'd', 'n', 'b', 'o', 'x', 'X', 'c':
	default:
		// Probed on 3.14: format(42, 'q') and format(42, 's').
		return nil, Raise(ValueError, "Unknown format code '%c' for object of type '%s'", sp.verb, typeName)
	}
	// Validation order probed on 3.14: precision beats z, format(42,
	// 'z.2') complains about precision; both beat the c-specific checks.
	if sp.hasPrec {
		return nil, Raise(ValueError, "Precision not allowed in integer format specifier")
	}
	if sp.coerceNeg {
		return nil, Raise(ValueError, "Negative zero coercion (z) not allowed in integer format specifier")
	}
	if sp.verb == 'c' {
		if sp.sign != 0 {
			// Probed on 3.14: format(42, '+c').
			return nil, Raise(ValueError, "Sign not allowed with integer format specifier 'c'")
		}
		if sp.alternate {
			// Probed on 3.14: format(42, '#c').
			return nil, Raise(ValueError, "Alternate form (#) not allowed with integer format specifier 'c'")
		}
		if x.big != nil || x.v < 0 || x.v > 0x10FFFF {
			// Probed on 3.14: format(-1, 'c') and format(0x110000, 'c').
			return nil, Raise(OverflowError, "%%c arg not in range(0x110000)")
		}
		return NewStr(buildNumber(sp, "", "", "", string(rune(x.v)), 3)), nil
	}
	base := 10
	groupSize := 3
	switch sp.verb {
	case 'b':
		base, groupSize = 2, 4
	case 'o':
		base, groupSize = 8, 4
	case 'x', 'X':
		base, groupSize = 16, 4
	}
	var neg bool
	var digits string
	if x.big != nil {
		// Probed: format(2**20000, 'd') hits the 4300-digit limit while
		// the binary-power bases stay exempt.
		if base == 10 {
			if _, derr := intDecimal(x); derr != nil {
				return nil, derr
			}
		}
		neg = x.big.Sign() < 0
		digits = new(big.Int).Abs(x.big).Text(base)
	} else {
		neg = x.v < 0
		u := uint64(x.v)
		if neg {
			u = -u
		}
		digits = strconv.FormatUint(u, base)
	}
	if sp.verb == 'X' {
		digits = strings.ToUpper(digits)
	}
	prefix := ""
	if sp.alternate {
		// Probed on 3.14: '#' shapes are 0b101010, 0o52, 0xff, 0XFF, and
		// '#d' adds nothing.
		switch sp.verb {
		case 'b':
			prefix = "0b"
		case 'o':
			prefix = "0o"
		case 'x':
			prefix = "0x"
		case 'X':
			prefix = "0X"
		}
	}
	return NewStr(buildNumber(sp, signStr(neg, sp.sign), prefix, digits, "", groupSize)), nil
}

// formatFloat handles the float presentations e E f F g G % n and none.
func formatFloat(v float64, typeName, spec string) (Object, error) {
	sp, err := parseFmtSpec(spec, typeName, 0)
	if err != nil {
		return nil, err
	}
	switch sp.verb {
	case 0, 'e', 'E', 'f', 'F', 'g', 'G', 'n', '%':
		return formatFloatSpec(v, sp)
	}
	// Probed on 3.14: format(3.14, 'd') and format(3.14, 'c') both take
	// this path.
	return nil, Raise(ValueError, "Unknown format code '%c' for object of type '%s'", sp.verb, typeName)
}

// formatFloatSpec renders a float under an already validated spec. The
// sign is managed here so 'z' coercion and '=' padding stay simple.
func formatFloatSpec(v float64, sp fmtSpec) (Object, error) {
	neg := math.Signbit(v)
	a := math.Abs(v)
	prec := sp.prec
	if !sp.hasPrec {
		prec = 6
	}
	var body string
	upper := false
	switch sp.verb {
	case 'f', 'F':
		body = fixedFloat(a, prec, sp.alternate)
		upper = sp.verb == 'F'
	case 'e', 'E':
		body = sciFloat(a, prec, sp.alternate)
		upper = sp.verb == 'E'
	case 'g', 'G', 'n':
		body = generalFloat(a, prec, sp.alternate, false)
		upper = sp.verb == 'G'
	case '%':
		// Probed on 3.14: format(0.5, '%') is '50.000000%' and infinities
		// keep the suffix, format(float('inf'), '%') is 'inf%'.
		body = fixedFloat(a*100, prec, sp.alternate) + "%"
	default:
		// Omitted type: repr-style shortest digits, or with a precision
		// the 'g' variant that switches to exponent one power sooner and
		// tacks '.0' onto bare integers. Probed on 3.14: format(2.5, '.1')
		// is '2e+00' where '.1g' gives '2', and format(100.0, '.4') is
		// '100.0'.
		if sp.hasPrec {
			body = generalFloat(a, prec, sp.alternate, true)
		} else {
			body = floatRepr(a)
			// The alternate form forces a point into an exponent-only
			// repr. Probed on 3.14: format(1e16, '#') is '1.e+16' while
			// format(123.0, '#') stays '123.0'.
			if sp.alternate && !strings.Contains(body, ".") {
				if i := strings.IndexByte(body, 'e'); i >= 0 {
					body = body[:i] + "." + body[i:]
				}
			}
		}
	}
	if sp.fracGroup != 0 {
		body = groupFraction(body, sp.fracGroup)
	}
	if upper {
		// 'E', 'F' and 'G' uppercase everything, inf and nan included.
		// Probed on 3.14: format(float('nan'), 'E') is 'NAN'.
		body = strings.ToUpper(body)
	}
	if sp.coerceNeg && neg && zeroDigitsOnly(body) {
		// The z flag drops the sign when the rounded value is a negative
		// zero. Probed on 3.14: format(-0.001, 'z.1f') is '0.0' while
		// format(-0.0005, 'z.3f') stays '-0.001'.
		neg = false
	}
	// Only the leading digit run groups with thousands separators, the
	// point, fraction, exponent or inf/nan tail rides along untouched.
	i := 0
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		i++
	}
	return NewStr(buildNumber(sp, signStr(neg, sp.sign), "", body[:i], body[i:], 3)), nil
}

// zeroDigitsOnly reports whether every digit in the mantissa of a
// rendered float is zero. Exponent digits do not count, and a body
// with no digits at all (inf, nan) keeps its sign under z.
func zeroDigitsOnly(body string) bool {
	seen := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == 'e' || c == 'E' {
			break
		}
		if c >= '0' && c <= '9' {
			if c != '0' {
				return false
			}
			seen = true
		}
	}
	return seen
}

// fixedFloat is the 'f' presentation of a non-negative value. The
// alternate form keeps a trailing point at precision zero. Probed on
// 3.14: format(1.0, '#.0f') is '1.'.
func fixedFloat(a float64, prec int, alt bool) string {
	if math.IsInf(a, 1) {
		return "inf"
	}
	if math.IsNaN(a) {
		return "nan"
	}
	s := strconv.FormatFloat(a, 'f', prec, 64)
	if alt && !strings.Contains(s, ".") {
		s += "."
	}
	return s
}

// sciFloat is the 'e' presentation of a non-negative value, with at
// least two exponent digits like CPython. Probed on 3.14:
// format(1.5, '#.0e') is '2.e+00'.
func sciFloat(a float64, prec int, alt bool) string {
	if math.IsInf(a, 1) {
		return "inf"
	}
	if math.IsNaN(a) {
		return "nan"
	}
	s := strconv.FormatFloat(a, 'e', prec, 64)
	if alt && prec == 0 {
		if i := strings.IndexByte(s, 'e'); i >= 0 {
			s = s[:i] + "." + s[i:]
		}
	}
	return s
}

// stripFloatZeros removes trailing fractional zeros and a then-bare
// point from a fixed-notation mantissa.
func stripFloatZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// generalFloat is the 'g' presentation of a non-negative value. A zero
// precision counts as one. noneMode is the omitted-type variant: the
// exponent cutoff drops by one and a result with no point or exponent
// gains '.0'. The alternate form keeps trailing zeros and the point.
// Probed on 3.14: format(1.0, '#g') is '1.00000' and format(1.0,
// '#.1g') is '1.'.
func generalFloat(a float64, prec int, alt, noneMode bool) string {
	if math.IsInf(a, 1) {
		return "inf"
	}
	if math.IsNaN(a) {
		return "nan"
	}
	if prec == 0 {
		prec = 1
	}
	es := strconv.FormatFloat(a, 'e', prec-1, 64)
	ei := strings.IndexByte(es, 'e')
	exp, _ := strconv.Atoi(es[ei+1:])
	cutoff := prec
	if noneMode {
		cutoff = prec - 1
	}
	if exp < -4 || exp >= cutoff {
		mant := es[:ei]
		if alt {
			if !strings.Contains(mant, ".") {
				mant += "."
			}
		} else {
			mant = stripFloatZeros(mant)
		}
		return mant + es[ei:]
	}
	s := strconv.FormatFloat(a, 'f', prec-1-exp, 64)
	if alt {
		if !strings.Contains(s, ".") {
			s += "."
		}
	} else {
		s = stripFloatZeros(s)
	}
	if noneMode && !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
