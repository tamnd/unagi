package objects

import (
	"math"
	"testing"
)

// Every want and wantErr below was probed on CPython 3.14 by evaluating
// the format % args expression shown in the case name; nothing here
// comes from memory. All cases run through the public Mod entry, the
// same path the emitted code takes.
func TestPercentFormat(t *testing.T) {
	inf := math.Inf(1)
	nan := math.NaN()
	kd := mustDict(NewStr("k"), NewInt(1))
	tests := []struct {
		name    string
		format  string
		arg     Object
		want    string
		wantErr string
	}{
		// %s and the right-operand shapes.
		{`"%s" % [1,2]`, "%s", L(NewInt(1), NewInt(2)), "[1, 2]", ""},
		{`"%s" % None`, "%s", None, "None", ""},
		{`"%s" % True`, "%s", True, "True", ""},
		{`"%s" % 42`, "%s", NewInt(42), "42", ""},
		{`"%s" % 3.14`, "%s", NewFloat(3.14), "3.14", ""},
		{`"%s" % 100.0`, "%s", NewFloat(100.0), "100.0", ""},
		{`"%s" % 1e16`, "%s", NewFloat(1e16), "1e+16", ""},
		{`"%s" % {"k": 1}`, "%s", kd, "{'k': 1}", ""},
		{`"%s" % ((1,2),)`, "%s", T(T(NewInt(1), NewInt(2))), "(1, 2)", ""},
		{`"%s" % range(3)`, "%s", NewRange(0, 3, 1), "range(0, 3)", ""},
		{`"%s%s%s" % (1,2,3)`, "%s%s%s", T(NewInt(1), NewInt(2), NewInt(3)), "123", ""},
		{`"a%sb%dc" % ("X",7)`, "a%sb%dc", T(NewStr("X"), NewInt(7)), "aXb7c", ""},
		{`"%10s" % "ab"`, "%10s", NewStr("ab"), "        ab", ""},
		{`"%-10s|" % "ab"`, "%-10s|", NewStr("ab"), "ab        |", ""},
		// The 0 flag is a space pad for strings.
		{`"%010s" % "ab"`, "%010s", NewStr("ab"), "        ab", ""},
		{`"%.3s" % "hello"`, "%.3s", NewStr("hello"), "hel", ""},
		{`"%5.2s" % "hello"`, "%5.2s", NewStr("hello"), "   he", ""},
		{`"%-5.2s|" % "hello"`, "%-5.2s|", NewStr("hello"), "he   |", ""},
		{`"%5.0s" % "ab"`, "%5.0s", NewStr("ab"), "     ", ""},
		// Sign and # flags are silently ignored on %s.
		{`"%+s" % 5`, "%+s", NewInt(5), "5", ""},
		{`"%#s" % 5`, "%#s", NewInt(5), "5", ""},

		// %r and %a.
		{`"%r" % "hi"`, "%r", NewStr("hi"), "'hi'", ""},
		{`"%r" % [1,"a"]`, "%r", L(NewInt(1), NewStr("a")), "[1, 'a']", ""},
		{`"%r" % True`, "%r", True, "True", ""},
		{`"%r" % None`, "%r", None, "None", ""},
		{`"%.2r" % "hello"`, "%.2r", NewStr("hello"), "'h", ""},
		{`"%a" % "héllo"`, "%a", NewStr("héllo"), `'h\xe9llo'`, ""},
		{`"%a" % "あ"`, "%a", NewStr("あ"), `'\u3042'`, ""},
		{`"%a" % "\U0001F600"`, "%a", NewStr("\U0001F600"), `'\U0001f600'`, ""},
		{`"%a" % [1, "é"]`, "%a", L(NewInt(1), NewStr("é")), `[1, '\xe9']`, ""},
		{`"%a" % 5`, "%a", NewInt(5), "5", ""},
		{`"%.2a" % "hello"`, "%.2a", NewStr("hello"), "'h", ""},

		// %d %i %u.
		{`"%d" % 3`, "%d", NewInt(3), "3", ""},
		{`"%i" % 42`, "%i", NewInt(42), "42", ""},
		{`"%u" % 42`, "%u", NewInt(42), "42", ""},
		{`"%d" % True`, "%d", True, "1", ""},
		{`"%d" % 3.7`, "%d", NewFloat(3.7), "3", ""},
		{`"%d" % -3.7`, "%d", NewFloat(-3.7), "-3", ""},
		{`"%d" % max`, "%d", NewInt(9223372036854775807), "9223372036854775807", ""},
		{`"%d" % min`, "%d", NewInt(-9223372036854775808), "-9223372036854775808", ""},
		{`"%.5d" % 42`, "%.5d", NewInt(42), "00042", ""},
		{`"%.5d" % -42`, "%.5d", NewInt(-42), "-00042", ""},
		{`"%05d" % -42`, "%05d", NewInt(-42), "-0042", ""},
		// Unlike C printf, the 0 flag still fills with a precision set.
		{`"%08.5d" % 42`, "%08.5d", NewInt(42), "00000042", ""},
		{`"%08.5d" % -42`, "%08.5d", NewInt(-42), "-0000042", ""},
		{`"%-05d" % 42`, "%-05d", NewInt(42), "42   ", ""},
		{`"%-.5d" % 42`, "%-.5d", NewInt(42), "00042", ""},
		{`"%+d" % 42`, "%+d", NewInt(42), "+42", ""},
		{`"%+d" % -42`, "%+d", NewInt(-42), "-42", ""},
		{`"% d" % 42`, "% d", NewInt(42), " 42", ""},
		{`"%010d" % 42`, "%010d", NewInt(42), "0000000042", ""},
		{`"%#d" % 42`, "%#d", NewInt(42), "42", ""},
		{`"%-+5d" % 42`, "%-+5d", NewInt(42), "+42  ", ""},
		{`"%+05d" % 42`, "%+05d", NewInt(42), "+0042", ""},
		{`"% +d" % 42`, "% +d", NewInt(42), "+42", ""},
		{`"%+ d" % 42`, "%+ d", NewInt(42), "+42", ""},
		{`"%0-5d" % 42`, "%0-5d", NewInt(42), "42   ", ""},
		{`"%--5d" % 42`, "%--5d", NewInt(42), "42   ", ""},
		{`"%++d" % 42`, "%++d", NewInt(42), "+42", ""},
		{`"%d" % "x"`, "%d", NewStr("x"), "", "TypeError: %d format: a real number is required, not str"},
		{`"%i" % "x"`, "%i", NewStr("x"), "", "TypeError: %i format: a real number is required, not str"},
		{`"%u" % "x"`, "%u", NewStr("x"), "", "TypeError: %u format: a real number is required, not str"},
		{`"%d" % [1]`, "%d", L(NewInt(1)), "", "TypeError: %d format: a real number is required, not list"},
		{`"%d" % None`, "%d", None, "", "TypeError: %d format: a real number is required, not NoneType"},
		{`"%d" % inf`, "%d", NewFloat(inf), "", "OverflowError: cannot convert float infinity to integer"},
		{`"%d" % nan`, "%d", NewFloat(nan), "", "ValueError: cannot convert float NaN to integer"},

		// %o %x %X.
		{`"%x" % 255`, "%x", NewInt(255), "ff", ""},
		{`"%X" % 255`, "%X", NewInt(255), "FF", ""},
		{`"%x" % -255`, "%x", NewInt(-255), "-ff", ""},
		{`"%o" % 8`, "%o", NewInt(8), "10", ""},
		{`"%o" % -8`, "%o", NewInt(-8), "-10", ""},
		{`"%#x" % 255`, "%#x", NewInt(255), "0xff", ""},
		{`"%#X" % 255`, "%#X", NewInt(255), "0XFF", ""},
		{`"%#o" % 8`, "%#o", NewInt(8), "0o10", ""},
		{`"%#o" % -8`, "%#o", NewInt(-8), "-0o10", ""},
		{`"%#x" % 0`, "%#x", NewInt(0), "0x0", ""},
		{`"%#o" % 0`, "%#o", NewInt(0), "0o0", ""},
		{`"%#x" % -255`, "%#x", NewInt(-255), "-0xff", ""},
		{`"%08x" % -255`, "%08x", NewInt(-255), "-00000ff", ""},
		{`"%#05x" % 255`, "%#05x", NewInt(255), "0x0ff", ""},
		{`"%-#7x" % 255`, "%-#7x", NewInt(255), "0xff   ", ""},
		{`"%0#7o" % 8`, "%0#7o", NewInt(8), "0o00010", ""},
		{`"%#5x" % 255`, "%#5x", NewInt(255), " 0xff", ""},
		{`"%.5x" % 255`, "%.5x", NewInt(255), "000ff", ""},
		{`"%#.5x" % 255`, "%#.5x", NewInt(255), "0x000ff", ""},
		{`"%#08.5x" % 255`, "%#08.5x", NewInt(255), "0x0000ff", ""},
		{`"%+x" % 255`, "%+x", NewInt(255), "+ff", ""},
		{`"% x" % 255`, "% x", NewInt(255), " ff", ""},
		{`"%+o" % 8`, "%+o", NewInt(8), "+10", ""},
		{`"%x" % True`, "%x", True, "1", ""},
		{`"%x" % False`, "%x", False, "0", ""},
		{`"%x" % max`, "%x", NewInt(9223372036854775807), "7fffffffffffffff", ""},
		{`"%o" % min`, "%o", NewInt(-9223372036854775808), "-1000000000000000000000", ""},
		{`"%x" % 3.7`, "%x", NewFloat(3.7), "", "TypeError: %x format: an integer is required, not float"},
		{`"%o" % 3.7`, "%o", NewFloat(3.7), "", "TypeError: %o format: an integer is required, not float"},
		{`"%X" % 1.5`, "%X", NewFloat(1.5), "", "TypeError: %X format: an integer is required, not float"},
		{`"%o" % "s"`, "%o", NewStr("s"), "", "TypeError: %o format: an integer is required, not str"},
		{`"%x" % None`, "%x", None, "", "TypeError: %x format: an integer is required, not NoneType"},

		// %f %F.
		{`"%f" % 3.14`, "%f", NewFloat(3.14), "3.140000", ""},
		{`"%f" % -0.0`, "%f", NewFloat(math.Copysign(0, -1)), "-0.000000", ""},
		{`"%.0f" % 2.5`, "%.0f", NewFloat(2.5), "2", ""},
		{`"%.0f" % 3.5`, "%.0f", NewFloat(3.5), "4", ""},
		{`"%.0f" % -2.5`, "%.0f", NewFloat(-2.5), "-2", ""},
		{`"%.0f" % 0.5`, "%.0f", NewFloat(0.5), "0", ""},
		{`"% .0f" % 0.5`, "% .0f", NewFloat(0.5), " 0", ""},
		{`"%.f" % 3.7`, "%.f", NewFloat(3.7), "4", ""},
		{`"%05.2f" % 3.14159`, "%05.2f", NewFloat(3.14159), "03.14", ""},
		{`"%08.3f" % -3.14159`, "%08.3f", NewFloat(-3.14159), "-003.142", ""},
		{`"%-8.3fX" % 3.14159`, "%-8.3fX", NewFloat(3.14159), "3.142   X", ""},
		{`"%+f" % 3.5`, "%+f", NewFloat(3.5), "+3.500000", ""},
		{`"% f" % 3.5`, "% f", NewFloat(3.5), " 3.500000", ""},
		{`"% 5.1f" % -0.05`, "% 5.1f", NewFloat(-0.05), " -0.1", ""},
		{`"%#.0f" % 3.0`, "%#.0f", NewFloat(3.0), "3.", ""},
		{`"%#5.0f" % 1.5`, "%#5.0f", NewFloat(1.5), "   2.", ""},
		{`"%#f" % 1.5`, "%#f", NewFloat(1.5), "1.500000", ""},
		{`"%f" % 1e-7`, "%f", NewFloat(1e-7), "0.000000", ""},
		{`"%.10f" % 1e-7`, "%.10f", NewFloat(1e-7), "0.0000001000", ""},
		{`"%f" % inf`, "%f", NewFloat(inf), "inf", ""},
		{`"%F" % inf`, "%F", NewFloat(inf), "INF", ""},
		{`"%f" % nan`, "%f", NewFloat(nan), "nan", ""},
		{`"%+f" % inf`, "%+f", NewFloat(inf), "+inf", ""},
		{`"% f" % nan`, "% f", NewFloat(nan), " nan", ""},
		// The 0 flag zero-pads even inf and nan.
		{`"%010f" % inf`, "%010f", NewFloat(inf), "0000000inf", ""},
		{`"%f" % "x"`, "%f", NewStr("x"), "", "TypeError: must be real number, not str"},

		// %e %E.
		{`"%e" % 0.0`, "%e", NewFloat(0.0), "0.000000e+00", ""},
		{`"%e" % -0.0`, "%e", NewFloat(math.Copysign(0, -1)), "-0.000000e+00", ""},
		{`"%e" % 1e100`, "%e", NewFloat(1e100), "1.000000e+100", ""},
		{`"%.0e" % 9.999e99`, "%.0e", NewFloat(9.999e99), "1e+100", ""},
		{`"%.0e" % 0.0`, "%.0e", NewFloat(0.0), "0e+00", ""},
		{`"%10.3e" % 1234.5678`, "%10.3e", NewFloat(1234.5678), " 1.235e+03", ""},
		{`"%010.2e" % 1234.5678`, "%010.2e", NewFloat(1234.5678), "001.23e+03", ""},
		{`"%+.3e" % 0.0001`, "%+.3e", NewFloat(0.0001), "+1.000e-04", ""},
		{`"%12.4e" % -1234.5678`, "%12.4e", NewFloat(-1234.5678), " -1.2346e+03", ""},
		{`"%.2e" % 999.99`, "%.2e", NewFloat(999.99), "1.00e+03", ""},
		{`"%.2e" % 0.000012345`, "%.2e", NewFloat(0.000012345), "1.23e-05", ""},
		{`"%E" % 1234567.891`, "%E", NewFloat(1234567.891), "1.234568E+06", ""},
		{`"%E" % inf`, "%E", NewFloat(inf), "INF", ""},
		{`"%010e" % nan`, "%010e", NewFloat(nan), "0000000nan", ""},
		{`"%#.0e" % 3.0`, "%#.0e", NewFloat(3.0), "3.e+00", ""},
		{`"%#e" % 1.0`, "%#e", NewFloat(1.0), "1.000000e+00", ""},
		{`"%e" % True`, "%e", True, "1.000000e+00", ""},
		{`"%e" % "x"`, "%e", NewStr("x"), "", "TypeError: must be real number, not str"},

		// %g %G.
		{`"%g" % 100000.0`, "%g", NewFloat(100000.0), "100000", ""},
		{`"%g" % 1e17`, "%g", NewFloat(1e17), "1e+17", ""},
		{`"%g" % 1e16`, "%g", NewFloat(1e16), "1e+16", ""},
		{`"%g" % 0.0001`, "%g", NewFloat(0.0001), "0.0001", ""},
		{`"%g" % 0.00001`, "%g", NewFloat(0.00001), "1e-05", ""},
		{`"%g" % 9.5e-5`, "%g", NewFloat(9.5e-5), "9.5e-05", ""},
		{`"%g" % 0.1`, "%g", NewFloat(0.1), "0.1", ""},
		{`"%g" % 999999.5`, "%g", NewFloat(999999.5), "1e+06", ""},
		{`"%g" % 1234567.0`, "%g", NewFloat(1234567.0), "1.23457e+06", ""},
		{`"%g" % 250000.0`, "%g", NewFloat(250000.0), "250000", ""},
		{`"%g" % 2500000.0`, "%g", NewFloat(2500000.0), "2.5e+06", ""},
		{`"%g" % -0.0`, "%g", NewFloat(math.Copysign(0, -1)), "-0", ""},
		{`"%g" % -inf`, "%g", NewFloat(math.Inf(-1)), "-inf", ""},
		{`"%g" % 42`, "%g", NewInt(42), "42", ""},
		{`"%.3g" % 1234.5678`, "%.3g", NewFloat(1234.5678), "1.23e+03", ""},
		{`"%.2g" % 99.9`, "%.2g", NewFloat(99.9), "1e+02", ""},
		{`"%.0g" % 3.14159`, "%.0g", NewFloat(3.14159), "3", ""},
		{`"%.1g" % 3.14159`, "%.1g", NewFloat(3.14159), "3", ""},
		{`"%.10g" % 3.14159`, "%.10g", NewFloat(3.14159), "3.14159", ""},
		{`"%.17g" % 0.1`, "%.17g", NewFloat(0.1), "0.10000000000000001", ""},
		{`"%G" % 1e-20`, "%G", NewFloat(1e-20), "1E-20", ""},
		{`"%G" % 123456789.0`, "%G", NewFloat(123456789.0), "1.23457E+08", ""},
		{`"%#g" % 3.0`, "%#g", NewFloat(3.0), "3.00000", ""},
		{`"%#.0g" % 3.0`, "%#.0g", NewFloat(3.0), "3.", ""},
		{`"%#G" % 0.0001234`, "%#G", NewFloat(0.0001234), "0.000123400", ""},
		{`"%05g" % 1.5`, "%05g", NewFloat(1.5), "001.5", ""},
		{`"%08g" % 123456789.0`, "%08g", NewFloat(123456789.0), "1.23457e+08", ""},
		{`"%15.4g" % 12345.6789`, "%15.4g", NewFloat(12345.6789), "      1.235e+04", ""},
		{`"%-15.4gX" % 12345.6789`, "%-15.4gX", NewFloat(12345.6789), "1.235e+04      X", ""},
		{`"%015.4g" % -12345.6789`, "%015.4g", NewFloat(-12345.6789), "-000001.235e+04", ""},
		{`"%g" % None`, "%g", None, "", "TypeError: must be real number, not NoneType"},

		// %c.
		{`"%c" % 65`, "%c", NewInt(65), "A", ""},
		{`"%c" % "A"`, "%c", NewStr("A"), "A", ""},
		{`"%c" % 0`, "%c", NewInt(0), "\x00", ""},
		{`"%c" % 3`, "%c", NewInt(3), "\x03", ""},
		{`"%c" % True`, "%c", True, "\x01", ""},
		{`"%c" % 0x10FFFF`, "%c", NewInt(0x10FFFF), "\U0010FFFF", ""},
		{`"%c" % "😀"`, "%c", NewStr("\U0001F600"), "\U0001F600", ""},
		{`"%5c" % 65`, "%5c", NewInt(65), "    A", ""},
		{`"%-5cX" % 65`, "%-5cX", NewInt(65), "A    X", ""},
		// The 0 flag is a space pad for %c and precision is ignored.
		{`"%05c" % 65`, "%05c", NewInt(65), "    A", ""},
		{`"%.3c" % 65`, "%.3c", NewInt(65), "A", ""},
		{`"%5c" % "あ"`, "%5c", NewStr("あ"), "    あ", ""},
		{`"%*c" % (5,65)`, "%*c", T(NewInt(5), NewInt(65)), "    A", ""},
		{`"%c" % "AB"`, "%c", NewStr("AB"), "", "TypeError: %c requires an int or a unicode character, not a string of length 2"},
		{`"%c" % ""`, "%c", NewStr(""), "", "TypeError: %c requires an int or a unicode character, not a string of length 0"},
		{`"%c" % 1114112`, "%c", NewInt(1114112), "", "OverflowError: %c arg not in range(0x110000)"},
		{`"%c" % -1`, "%c", NewInt(-1), "", "OverflowError: %c arg not in range(0x110000)"},
		{`"%c" % 3.5`, "%c", NewFloat(3.5), "", "TypeError: %c requires an int or a unicode character, not float"},
		{`"%c" % []`, "%c", L(), "", "TypeError: %c requires an int or a unicode character, not list"},

		// %% literal and incomplete formats.
		{`"%d%%" % 5`, "%d%%", NewInt(5), "5%", ""},
		{`"%%" % ()`, "%%", T(), "%", ""},
		{`"" % ()`, "", T(), "", ""},
		{`"%" % 5`, "%", NewInt(5), "", "ValueError: incomplete format"},
		{`"abc%" % ()`, "abc%", T(), "", "ValueError: incomplete format"},
		{`"%d%" % 5`, "%d%", NewInt(5), "", "ValueError: incomplete format"},
		// A width kills the literal escape and the argument fetch runs
		// before the character check.
		{`"%5%" % (1,)`, "%5%", T(NewInt(1)), "", "ValueError: unsupported format character '%' (0x25) at index 2"},
		{`"%5%" % ()`, "%5%", T(), "", "TypeError: not enough arguments for format string"},
		{`"%*%" % (5,)`, "%*%", T(NewInt(5)), "", "TypeError: not enough arguments for format string"},
		{`"%%" % 5`, "%%", NewInt(5), "", "TypeError: not all arguments converted during string formatting"},

		// Argument count accounting.
		{`"%s %s" % "ab"`, "%s %s", NewStr("ab"), "", "TypeError: not enough arguments for format string"},
		{`"%s %s" % (1,)`, "%s %s", T(NewInt(1)), "", "TypeError: not enough arguments for format string"},
		{`"%d" % ()`, "%d", T(), "", "TypeError: not enough arguments for format string"},
		{`"%s" % (1,2)`, "%s", T(NewInt(1), NewInt(2)), "", "TypeError: not all arguments converted during string formatting"},
		{`"" % 5`, "", NewInt(5), "", "TypeError: not all arguments converted during string formatting"},
		{`"" % "ab"`, "", NewStr("ab"), "", "TypeError: not all arguments converted during string formatting"},
		// A mapping right operand suppresses the leftover check.
		{`"" % [1]`, "", L(NewInt(1)), "", ""},
		{`"" % {"a":1}`, "", mustDict(NewStr("a"), NewInt(1)), "", ""},
		{`"" % range(3)`, "", NewRange(0, 3, 1), "", ""},

		// Star width and precision.
		{`"%*d" % (5,42)`, "%*d", T(NewInt(5), NewInt(42)), "   42", ""},
		{`"%*d" % (-5,42)`, "%*d", T(NewInt(-5), NewInt(42)), "42   ", ""},
		{`"%*s" % (5,"ab")`, "%*s", T(NewInt(5), NewStr("ab")), "   ab", ""},
		{`"%.*f" % (2,3.14159)`, "%.*f", T(NewInt(2), NewFloat(3.14159)), "3.14", ""},
		// A negative star precision clamps to zero.
		{`"%.*f" % (-2,3.14159)`, "%.*f", T(NewInt(-2), NewFloat(3.14159)), "3", ""},
		{`"%.*s" % (-2,"hello")`, "%.*s", T(NewInt(-2), NewStr("hello")), "", ""},
		{`"%.*g" % (-2,3.14159)`, "%.*g", T(NewInt(-2), NewFloat(3.14159)), "3", ""},
		{`"%*.*s" % (10,2,"hello")`, "%*.*s", T(NewInt(10), NewInt(2), NewStr("hello")), "        he", ""},
		{`"%*d" % (True,5)`, "%*d", T(True, NewInt(5)), "5", ""},
		{`"%.*d" % (True,42)`, "%.*d", T(True, NewInt(42)), "42", ""},
		{`"%*d" % ("x",5)`, "%*d", T(NewStr("x"), NewInt(5)), "", "TypeError: * wants int"},
		{`"%*d" % (5.0,42)`, "%*d", T(NewFloat(5.0), NewInt(42)), "", "TypeError: * wants int"},
		{`"%.*f" % ("x",5)`, "%.*f", T(NewStr("x"), NewInt(5)), "", "TypeError: * wants int"},

		// The %(key)s dict form.
		{`"%(k)s" % {"k":1}`, "%(k)s", kd, "1", ""},
		{`"%(k)s" % {1:"x","k":9}`, "%(k)s", mustDict(NewInt(1), NewStr("x"), NewStr("k"), NewInt(9)), "9", ""},
		{`"%(k)s and %(j)r"`, "%(k)s and %(j)r", mustDict(NewStr("k"), NewInt(1), NewStr("j"), NewStr("x")), "1 and 'x'", ""},
		{`"%(k)s %(k)r" % {"k":"v"}`, "%(k)s %(k)r", mustDict(NewStr("k"), NewStr("v")), "v 'v'", ""},
		{`"%(k)10.2f"`, "%(k)10.2f", mustDict(NewStr("k"), NewFloat(3.14159)), "      3.14", ""},
		{`"%(k)#x" % {"k":255}`, "%(k)#x", mustDict(NewStr("k"), NewInt(255)), "0xff", ""},
		{`"%(k)c" % {"k":65}`, "%(k)c", mustDict(NewStr("k"), NewInt(65)), "A", ""},
		{`"%(k)s" % {"k":[1,2]}`, "%(k)s", mustDict(NewStr("k"), L(NewInt(1), NewInt(2))), "[1, 2]", ""},
		{`"%(k)s %%" % {"k":1}`, "%(k)s %%", kd, "1 %", ""},
		// Keys nest parens and can be empty or blank.
		{`"%(a(b)c)s"`, "%(a(b)c)s", mustDict(NewStr("a(b)c"), NewInt(1)), "1", ""},
		{`"%((a)b)s"`, "%((a)b)s", mustDict(NewStr("(a)b"), NewInt(3)), "3", ""},
		{`"%()s" % {"":7}`, "%()s", mustDict(NewStr(""), NewInt(7)), "7", ""},
		{`"%( )s" % {" ":8}`, "%( )s", mustDict(NewStr(" "), NewInt(8)), "8", ""},
		{`"%(k)s" % 5`, "%(k)s", NewInt(5), "", "TypeError: format requires a mapping"},
		{`"%(k)s" % "abc"`, "%(k)s", NewStr("abc"), "", "TypeError: format requires a mapping"},
		{`"%(k)s" % (1,)`, "%(k)s", T(NewInt(1)), "", "TypeError: format requires a mapping"},
		// The mapping check runs before the key scan.
		{`"%(k" % 5`, "%(k", NewInt(5), "", "TypeError: format requires a mapping"},
		{`"%(k)s" % {"j":1}`, "%(k)s", mustDict(NewStr("j"), NewInt(1)), "", "KeyError: 'k'"},
		{`"%(k" % {"k":1}`, "%(k", kd, "", "ValueError: incomplete format key"},
		// A named lookup resets the positional cursor to just the value.
		{`"%(k)s %s" % {"k":1}`, "%(k)s %s", kd, "", "TypeError: not enough arguments for format string"},
		{`"%(k)*d" % {"k":42}`, "%(k)*d", mustDict(NewStr("k"), NewInt(42)), "", "TypeError: not enough arguments for format string"},
		// A positional conversion on a dict right operand grabs the dict.
		{`"%d %(k)s" % {"k":1}`, "%d %(k)s", kd, "", "TypeError: %d format: a real number is required, not dict"},
		{`"%(k)s" % [1]`, "%(k)s", L(NewInt(1)), "", "TypeError: list indices must be integers or slices, not str"},
		{`"%(k)s" % range(3)`, "%(k)s", NewRange(0, 3, 1), "", "TypeError: range indices must be integers or slices, not str"},

		// Unsupported conversion characters.
		{`"%y" % 5`, "%y", NewInt(5), "", "ValueError: unsupported format character 'y' (0x79) at index 1"},
		{`"ab%yz" % 5`, "ab%yz", NewInt(5), "", "ValueError: unsupported format character 'y' (0x79) at index 3"},
		{`"%b" % 5`, "%b", NewInt(5), "", "ValueError: unsupported format character 'b' (0x62) at index 1"},
		{`"%q" % ("a",)`, "%q", T(NewStr("a")), "", "ValueError: unsupported format character 'q' (0x71) at index 1"},
		// Non-ASCII characters show as '?'; the index counts code points.
		{`"%é" % 5`, "%é", NewInt(5), "", "ValueError: unsupported format character '?' (0xe9) at index 1"},
		// The fetch happens first, so no args wins over the bad char.
		{`"%y" % ()`, "%y", T(), "", "TypeError: not enough arguments for format string"},
		{`"%y" % {"k":1}`, "%y", kd, "", "ValueError: unsupported format character 'y' (0x79) at index 1"},
		{`"%(k)5y" % {"k":1}`, "%(k)5y", kd, "", "ValueError: unsupported format character 'y' (0x79) at index 5"},
		// A key is only parsed right after the %, not after flags.
		{`"%-(k)s" % {"k":1}`, "%-(k)s", kd, "", "ValueError: unsupported format character '(' (0x28) at index 2"},

		// One C length modifier is skipped, a second one is an error.
		{`"%ld" % 5`, "%ld", NewInt(5), "5", ""},
		{`"%hd" % 5`, "%hd", NewInt(5), "5", ""},
		{`"%Ld" % 5`, "%Ld", NewInt(5), "5", ""},
		{`"%lld" % 5`, "%lld", NewInt(5), "", "ValueError: unsupported format character 'l' (0x6c) at index 2"},
		{`"%hhd" % 5`, "%hhd", NewInt(5), "", "ValueError: unsupported format character 'h' (0x68) at index 2"},
	}
	for _, tt := range tests {
		got, err := Mod(NewStr(tt.format), tt.arg)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		s, ok := AsStr(got)
		if !ok {
			t.Errorf("%s: result is not a str: %v", tt.name, got)
			continue
		}
		if s != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, s, tt.want)
		}
	}
}

// TestPercentModDispatch pins the Mod wiring: a str left operand takes
// the formatting path and numeric mod still works.
func TestPercentModDispatch(t *testing.T) {
	got, err := Mod(NewStr("%d"), NewInt(3))
	if err != nil {
		t.Fatalf("Mod str: unexpected error %v", err)
	}
	if s, _ := AsStr(got); s != "3" {
		t.Errorf("Mod str: got %q, want %q", s, "3")
	}
	got, err = Mod(NewInt(7), NewInt(3))
	if err != nil {
		t.Fatalf("Mod int: unexpected error %v", err)
	}
	if Repr(got) != "1" {
		t.Errorf("Mod int: got %s, want 1", Repr(got))
	}
	if _, err = Mod(NewInt(1), NewStr("x")); err == nil ||
		err.Error() != "TypeError: unsupported operand type(s) for %: 'int' and 'str'" {
		t.Errorf("Mod int/str: got %v", err)
	}
}
