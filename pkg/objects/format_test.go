package objects

import (
	"math"
	"testing"
)

// fmtCase drives Format through one value/spec pair. Every expected
// value and error text below was probed on CPython 3.14 with
// format(value, spec); the comments call out the less obvious probes.
type fmtCase struct {
	name    string
	o       Object
	spec    string
	want    string
	wantErr string
}

func runFmtCases(t *testing.T, tests []fmtCase) {
	t.Helper()
	for _, tt := range tests {
		got, err := Format(tt.o, tt.spec)
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

func TestFormatStr(t *testing.T) {
	runFmtCases(t, []fmtCase{
		{"empty spec", NewStr("abc"), "", "abc", ""},
		{"width", NewStr("abc"), "5", "abc  ", ""},
		{"left", NewStr("abc"), "<5", "abc  ", ""},
		{"right", NewStr("abc"), ">5", "  abc", ""},
		// Probed on 3.14: the odd fill char of a centered value goes right.
		{"center", NewStr("abc"), "^6", " abc  ", ""},
		{"center fill", NewStr("abc"), "*^7", "**abc**", ""},
		{"truncate", NewStr("abc"), ".2", "ab", ""},
		{"truncate to nothing", NewStr("abc"), ".0", "", ""},
		{"truncate then pad", NewStr("abc"), "10.2", "ab        ", ""},
		{"s code", NewStr("abc"), "s", "abc", ""},
		{"s code padded", NewStr("abc"), ">5s", "  abc", ""},
		// Probed on 3.14: '0' keeps the '<' default for str, so the zeros
		// land on the right.
		{"zero pad stays left", NewStr("abc"), "05", "abc00", ""},
		{"bare zero", NewStr("abc"), "0", "abc", ""},
		{"explicit zero fill", NewStr("ab"), "0>5", "000ab", ""},
		{"unicode fill", NewStr("abc"), "é^5", "éabcé", ""},
		{"rune truncation", NewStr("héllo"), ".2", "hé", ""},
		{"rune width", NewStr("héllo"), "7", "héllo  ", ""},
		{"empty value", NewStr(""), "5", "     ", ""},
		// Probed on 3.14: a fractional separator after the precision is
		// accepted and ignored for str.
		{"frac group ignored", NewStr("abc"), ".2_", "ab", ""},
		{"frac group ignored comma", NewStr("abcdef"), ".4,", "abcd", ""},

		{"eq align", NewStr("abc"), "=5", "", "ValueError: '=' alignment not allowed in string format specifier"},
		{"zero eq align", NewStr("abc"), "0=5", "", "ValueError: '=' alignment not allowed in string format specifier"},
		{"plus", NewStr("abc"), "+", "", "ValueError: Sign not allowed in string format specifier"},
		{"minus", NewStr("abc"), "-", "", "ValueError: Sign not allowed in string format specifier"},
		// Probed on 3.14: the space sign has its own wording.
		{"space", NewStr("abc"), " ", "", "ValueError: Space not allowed in string format specifier"},
		{"comma", NewStr("abc"), ",", "", "ValueError: Cannot specify ',' with 's'."},
		{"underscore", NewStr("abc"), "_", "", "ValueError: Cannot specify '_' with 's'."},
		{"hash", NewStr("abc"), "#", "", "ValueError: Alternate form (#) not allowed in string format specifier"},
		{"z flag", NewStr("abc"), "z", "", "ValueError: Negative zero coercion (z) not allowed in string format specifier"},
		{"unknown d", NewStr("abc"), "d", "", "ValueError: Unknown format code 'd' for object of type 'str'"},
		{"unknown q", NewStr("abc"), "q", "", "ValueError: Unknown format code 'q' for object of type 'str'"},
		// Probed on 3.14: a stray brace is an unknown code, not an invalid
		// specifier.
		{"brace", NewStr("x"), "{", "", "ValueError: Unknown format code '{' for object of type 'str'"},
		// Probed on 3.14: the unknown-code check beats the flag checks.
		{"sign with unknown", NewStr("abc"), "+d", "", "ValueError: Unknown format code 'd' for object of type 'str'"},
		// Probed on 3.14: sign beats '#', z beats '#'.
		{"sign before hash", NewStr("abc"), "+#", "", "ValueError: Sign not allowed in string format specifier"},
		{"z before hash", NewStr("abc"), "z#", "", "ValueError: Negative zero coercion (z) not allowed in string format specifier"},
		{"comma before sign", NewStr("abc"), "+,", "", "ValueError: Cannot specify ',' with 's'."},
		{"invalid", NewStr("x"), "ss", "", "ValueError: Invalid format specifier 'ss' for object of type 'str'"},
		{"invalid with z", NewStr("a"), "z d", "", "ValueError: Invalid format specifier 'z d' for object of type 'str'"},
		{"missing precision", NewStr("a"), ".", "", "ValueError: Format specifier missing precision"},
	})
}

func TestFormatInt(t *testing.T) {
	runFmtCases(t, []fmtCase{
		{"empty spec", NewInt(42), "", "42", ""},
		{"d", NewInt(42), "d", "42", ""},
		{"width", NewInt(42), "5", "   42", ""},
		{"left", NewInt(42), "<5", "42   ", ""},
		{"center", NewInt(42), "^5", " 42  ", ""},
		{"eq default fill", NewInt(42), "=5", "   42", ""},
		{"eq star fill", NewInt(42), "*=6", "****42", ""},
		// Probed on 3.14: '=' puts the padding after the sign.
		{"eq negative", NewInt(-42), "=6", "-   42", ""},
		{"zero pad negative", NewInt(-42), "06", "-00042", ""},
		{"zero pad flag width", NewInt(-5), "003", "-05", ""},
		{"plus", NewInt(42), "+", "+42", ""},
		{"minus", NewInt(42), "-", "42", ""},
		{"space", NewInt(42), " ", " 42", ""},
		{"space negative", NewInt(-42), " ", "-42", ""},
		{"plus zero pad", NewInt(42), "+06", "+00042", ""},
		{"sign right align", NewInt(-42), "*>+7", "****-42", ""},
		{"eq sign star", NewInt(42), "*=+7", "+****42", ""},
		{"binary", NewInt(42), "b", "101010", ""},
		{"octal", NewInt(42), "o", "52", ""},
		{"hex", NewInt(42), "x", "2a", ""},
		{"hex upper", NewInt(255), "X", "FF", ""},
		{"alt hex", NewInt(255), "#x", "0xff", ""},
		{"alt hex upper", NewInt(255), "#X", "0XFF", ""},
		{"alt binary", NewInt(42), "#b", "0b101010", ""},
		{"alt octal", NewInt(42), "#o", "0o52", ""},
		{"alt decimal", NewInt(42), "#d", "42", ""},
		{"alt hex negative", NewInt(-42), "#x", "-0x2a", ""},
		// Probed on 3.14: zeros go between the prefix and the digits.
		{"alt hex zero pad", NewInt(-42), "#06x", "-0x02a", ""},
		// Probed on 3.14: '=' fill goes between the prefix and the digits
		// too, format(-42, '*=#8x') is '-0x***2a'.
		{"alt hex eq fill", NewInt(-42), "*=#8x", "-0x***2a", ""},
		{"c", NewInt(97), "c", "a", ""},
		{"c star", NewInt(42), "c", "*", ""},
		{"c wide rune", NewInt(0x65e5), "c", "日", ""},
		{"c nul", NewInt(0), "c", "\x00", ""},
		{"c padded", NewInt(97), "5c", "    a", ""},
		{"c zero padded", NewInt(97), "05c", "0000a", ""},
		{"c eq align", NewInt(97), "=5c", "    a", ""},
		{"comma", NewInt(1234567), ",", "1,234,567", ""},
		{"underscore", NewInt(1234567), "_", "1_234_567", ""},
		{"comma d", NewInt(1234567), ",d", "1,234,567", ""},
		{"comma negative", NewInt(-1234567), ",", "-1,234,567", ""},
		// Probed on 3.14: binary, octal and hex group every four digits.
		{"underscore binary", NewInt(1234567), "_b", "1_0010_1101_0110_1000_0111", ""},
		{"underscore hex", NewInt(43981), "_X", "ABCD", ""},
		{"underscore octal", NewInt(1234567890), "_o", "111_4540_1322", ""},
		// Zero-padding under grouping extends the digits so no separator
		// leads, overshooting the width when needed. Probed on 3.14:
		// format(1234, '08,') is nine characters.
		{"zero group w5", NewInt(1234), "05,", "1,234", ""},
		{"zero group w6", NewInt(1234), "06,", "01,234", ""},
		{"zero group w7", NewInt(1234), "07,", "001,234", ""},
		{"zero group w8", NewInt(1234), "08,", "0,001,234", ""},
		{"zero group w9", NewInt(1234), "09,", "0,001,234", ""},
		{"zero group w10", NewInt(1234), "010,", "00,001,234", ""},
		{"zero group w12", NewInt(1234), "012,", "0,000,001,234", ""},
		{"zero group negative", NewInt(-1234), "08,", "-001,234", ""},
		{"zero group zero", NewInt(0), "04,", "0,000", ""},
		{"zero group hex", NewInt(255), "#010_x", "0x000_00ff", ""},
		{"zero group hex wider", NewInt(255), "#012_x", "0x0_0000_00ff", ""},
		{"zero group binary", NewInt(5), "08_b", "000_0101", ""},
		// Probed on 3.14: grouping with a non-zero '=' fill pads plainly.
		{"star eq group", NewInt(1234), "*=8,", "***1,234", ""},
		{"group right align", NewInt(1234), "8,", "   1,234", ""},
		{"unicode fill", NewInt(42), "日>4", "日日42", ""},
		{"float code f", NewInt(42), ".2f", "42.00", ""},
		{"float code e", NewInt(42), "e", "4.200000e+01", ""},
		{"float code g", NewInt(42), "g", "42", ""},
		{"float code percent", NewInt(42), "%", "4200.000000%", ""},
		// Probed on 3.14: the float rules apply wholesale on delegation,
		// z and '#' included.
		{"float code full flags", NewInt(42), "z=+#,.2f", "+42.00", ""},
		{"zero", NewInt(0), "", "0", ""},
		{"zero alt hex", NewInt(0), "#x", "0x0", ""},
		{"n code", NewInt(1234567), "n", "1234567", ""},

		{"precision", NewInt(42), ".2", "", "ValueError: Precision not allowed in integer format specifier"},
		{"precision d", NewInt(42), ".2d", "", "ValueError: Precision not allowed in integer format specifier"},
		{"precision c", NewInt(97), ".1c", "", "ValueError: Precision not allowed in integer format specifier"},
		// Probed on 3.14: precision beats the z check.
		{"precision before z", NewInt(42), "z.2", "", "ValueError: Precision not allowed in integer format specifier"},
		{"z flag", NewInt(42), "z", "", "ValueError: Negative zero coercion (z) not allowed in integer format specifier"},
		{"z before comma", NewInt(42), "z,", "", "ValueError: Negative zero coercion (z) not allowed in integer format specifier"},
		{"z before c checks", NewInt(97), "zc", "", "ValueError: Negative zero coercion (z) not allowed in integer format specifier"},
		{"unknown q", NewInt(42), "q", "", "ValueError: Unknown format code 'q' for object of type 'int'"},
		{"unknown s", NewInt(42), "s", "", "ValueError: Unknown format code 's' for object of type 'int'"},
		// Probed on 3.14: dispatch on the code beats the precision check.
		{"unknown before precision", NewInt(42), ".2q", "", "ValueError: Unknown format code 'q' for object of type 'int'"},
		{"unknown plus", NewInt(42), "++", "", "ValueError: Unknown format code '+' for object of type 'int'"},
		{"invalid", NewInt(3), "q q", "", "ValueError: Invalid format specifier 'q q' for object of type 'int'"},
		{"invalid qq", NewInt(42), "qq", "", "ValueError: Invalid format specifier 'qq' for object of type 'int'"},
		// Probed on 3.14: '<<' parses as fill '<' align '<', a third one
		// is left over and lands in the type slot.
		{"fill align same", NewInt(42), "<<", "42", ""},
		{"fill align extra", NewInt(42), "<<<", "", "ValueError: Unknown format code '<' for object of type 'int'"},
		{"comma hex", NewInt(255), ",x", "", "ValueError: Cannot specify ',' with 'x'."},
		{"comma binary", NewInt(255), ",b", "", "ValueError: Cannot specify ',' with 'b'."},
		{"comma octal", NewInt(255), ",o", "", "ValueError: Cannot specify ',' with 'o'."},
		{"comma c", NewInt(42), ",c", "", "ValueError: Cannot specify ',' with 'c'."},
		{"comma n", NewInt(42), ",n", "", "ValueError: Cannot specify ',' with 'n'."},
		{"underscore n", NewInt(42), "_n", "", "ValueError: Cannot specify '_' with 'n'."},
		{"comma unknown", NewInt(42), ",q", "", "ValueError: Cannot specify ',' with 'q'."},
		// Probed on 3.14: format(42, ',,') reads the second comma as the
		// type code.
		{"comma comma", NewInt(42), ",,", "", "ValueError: Cannot specify ',' with ','."},
		{"comma before precision", NewInt(42), ",.2q", "", "ValueError: Cannot specify ',' with 'q'."},
		{"both separators", NewInt(1234), ",_", "", "ValueError: Cannot specify both ',' and '_'."},
		{"both separators reversed", NewInt(1234), "_,", "", "ValueError: Cannot specify both ',' and '_'."},
		{"underscore underscore", NewInt(1234), "__", "", "ValueError: Cannot specify '_' with '_'."},
		{"c sign", NewInt(42), "+c", "", "ValueError: Sign not allowed with integer format specifier 'c'"},
		{"c alternate", NewInt(42), "#c", "", "ValueError: Alternate form (#) not allowed with integer format specifier 'c'"},
		{"c underscore", NewInt(42), "_c", "", "ValueError: Cannot specify '_' with 'c'."},
		{"c negative", NewInt(-1), "c", "", "OverflowError: %c arg not in range(0x110000)"},
		{"c too big", NewInt(0x110000), "c", "", "OverflowError: %c arg not in range(0x110000)"},
		{"missing precision", NewInt(42), ".d", "", "ValueError: Format specifier missing precision"},
	})
}

func TestFormatFloat(t *testing.T) {
	runFmtCases(t, []fmtCase{
		// Probed on 3.14: the empty spec matches str().
		{"empty spec", NewFloat(3.14159), "", "3.14159", ""},
		{"empty adds dot zero", NewFloat(1), "", "1.0", ""},
		{"empty big fixed", NewFloat(2e15), "", "2000000000000000.0", ""},
		{"empty big exp", NewFloat(1e16), "", "1e+16", ""},
		{"empty small", NewFloat(1e-4), "", "0.0001", ""},
		{"empty smaller", NewFloat(1e-5), "", "1e-05", ""},
		{"empty negative zero", NewFloat(math.Copysign(0, -1)), "", "-0.0", ""},
		{"f", NewFloat(3.14159), "f", "3.141590", ""},
		{"f precision", NewFloat(3.14159), ".2f", "3.14", ""},
		{"f zero precision", NewFloat(3.14159), ".0f", "3", ""},
		// Probed on 3.14: ties round to even.
		{"f round half even", NewFloat(2.5), ".0f", "2", ""},
		{"f round half odd", NewFloat(3.5), ".0f", "4", ""},
		{"e", NewFloat(3.14159), "e", "3.141590e+00", ""},
		{"E", NewFloat(3.14159), "E", "3.141590E+00", ""},
		{"e precision", NewFloat(3.14159), ".2e", "3.14e+00", ""},
		{"e big exponent", NewFloat(1e100), "e", "1.000000e+100", ""},
		{"e negative exponent", NewFloat(1e-5), "e", "1.000000e-05", ""},
		{"g", NewFloat(3.14159), "g", "3.14159", ""},
		{"g precision", NewFloat(3.14159), ".3g", "3.14", ""},
		{"g exp form", NewFloat(1234567.0), "g", "1.23457e+06", ""},
		{"g small", NewFloat(0.000012345), "g", "1.2345e-05", ""},
		{"g strips zeros", NewFloat(100.0), "g", "100", ""},
		{"g zero", NewFloat(0), "g", "0", ""},
		// Probed on 3.14: 'g' treats precision 0 as 1.
		{"g zero precision", NewFloat(123.456), ".0g", "1e+02", ""},
		{"G upper", NewFloat(2.5e-10), "G", "2.5E-10", ""},
		{"percent", NewFloat(0.5), "%", "50.000000%", ""},
		{"percent precision", NewFloat(0.5), ".1%", "50.0%", ""},
		{"percent zero precision", NewFloat(0.25), ".0%", "25%", ""},
		{"width", NewFloat(1.5), "10.2f", "      1.50", ""},
		{"zero pad", NewFloat(-1.5), "010.2f", "-000001.50", ""},
		{"eq pad", NewFloat(-1.5), "=10.2f", "-     1.50", ""},
		{"center", NewFloat(-1.5), "^8.1f", "  -1.5  ", ""},
		{"plus", NewFloat(1.5), "+.2f", "+1.50", ""},
		{"space", NewFloat(1.5), " .2f", " 1.50", ""},
		// Probed on 3.14: 'F' uppercases inf and nan, 'f' does not.
		{"inf f", NewFloat(math.Inf(1)), "f", "inf", ""},
		{"inf F", NewFloat(math.Inf(1)), "F", "INF", ""},
		{"neg inf F", NewFloat(math.Inf(-1)), "F", "-INF", ""},
		{"nan f", NewFloat(math.NaN()), "f", "nan", ""},
		{"nan F", NewFloat(math.NaN()), "F", "NAN", ""},
		{"nan E", NewFloat(math.NaN()), "E", "NAN", ""},
		{"inf e", NewFloat(math.Inf(1)), "e", "inf", ""},
		{"inf empty", NewFloat(math.Inf(1)), "", "inf", ""},
		{"inf percent", NewFloat(math.Inf(1)), "%", "inf%", ""},
		{"nan padded", NewFloat(math.NaN()), "10.2f", "       nan", ""},
		// Probed on 3.14: zero fill runs into a non-finite body plainly,
		// no separators.
		{"inf zero pad", NewFloat(math.Inf(1)), "010.2f", "0000000inf", ""},
		{"inf zero group pad", NewFloat(math.Inf(1)), "010,.1f", "0000000inf", ""},
		{"inf plus", NewFloat(math.Inf(1)), "+.2f", "+inf", ""},
		{"comma", NewFloat(1234567.891), ",", "1,234,567.891", ""},
		{"comma f", NewFloat(1234567.891), ",.2f", "1,234,567.89", ""},
		{"underscore f", NewFloat(1234567.891), "_.2f", "1_234_567.89", ""},
		{"comma g", NewFloat(123456.789), "_g", "123_457", ""},
		{"comma percent", NewFloat(12345.678), ",%", "1,234,567.800000%", ""},
		{"comma repr mode", NewFloat(12345.0), ",", "12,345.0", ""},
		// Zero padding under grouping extends the integer digits only.
		// Probed on 3.14: format(1234.5, '012,.1f').
		{"zero group pad", NewFloat(1234.5), "012,.1f", "00,001,234.5", ""},
		{"zero group pad wider", NewFloat(1234.5), "014,.1f", "0,000,001,234.5", ""},
		{"zero group pad negative", NewFloat(-1234.5), "012,.1f", "-0,001,234.5", ""},
		// The omitted type with a precision flips to the exponent one
		// power sooner than 'g' and pins '.0' on bare integers. Probed on
		// 3.14: format(2.5, '.1') is '2e+00' where '.1g' gives '2'.
		{"none precision exp", NewFloat(2.5), ".1", "2e+00", ""},
		{"none precision dot zero", NewFloat(100.0), ".4", "100.0", ""},
		{"none precision fixed", NewFloat(3.14159), ".4", "3.142", ""},
		{"none precision exp big", NewFloat(31415.9), ".4", "3.142e+04", ""},
		{"none precision huge", NewFloat(1e16), ".17", "1e+16", ""},
		{"none precision long", NewFloat(0.1), ".20", "0.10000000000000000555", ""},
		{"g long", NewFloat(0.1), ".30g", "0.100000000000000005551115123126", ""},
		// The z flag drops the sign of a rounded-to-zero result. Probed
		// on 3.14: format(-0.001, 'z.1f') is '0.0'.
		{"z coerce", NewFloat(-0.001), "z.1f", "0.0", ""},
		{"z coerce e", NewFloat(math.Copysign(0, -1)), "z.1e", "0.0e+00", ""},
		{"z coerce g", NewFloat(math.Copysign(0, -1)), "zg", "0", ""},
		{"z keeps nonzero", NewFloat(-1e-10), "z.2e", "-1.00e-10", ""},
		{"z keeps rounded away", NewFloat(-0.0005), "z.3f", "-0.001", ""},
		{"z half even", NewFloat(-0.5), "z.0f", "0", ""},
		{"z no effect", NewFloat(1.5), "z", "1.5", ""},
		{"z inf keeps sign", NewFloat(math.Inf(-1)), "zf", "-inf", ""},
		{"negative zero f", NewFloat(math.Copysign(0, -1)), "f", "-0.000000", ""},
		// Alternate form keeps points and zeros. Probed on 3.14.
		{"alt f", NewFloat(1), "#.0f", "1.", ""},
		{"alt e", NewFloat(1.5), "#.0e", "2.e+00", ""},
		{"alt g", NewFloat(1), "#g", "1.00000", ""},
		{"alt g precision", NewFloat(1), "#.3g", "1.00", ""},
		{"alt g one digit", NewFloat(1), "#.1g", "1.", ""},
		{"alt g exp", NewFloat(1234567.0), "#.3g", "1.23e+06", ""},
		{"alt none", NewFloat(3.14), "#", "3.14", ""},
		{"alt none precision", NewFloat(1), "#.4", "1.000", ""},
		// Probed on 3.14: format(1e16, '#') is '1.e+16'.
		{"alt repr exp", NewFloat(1e16), "#", "1.e+16", ""},
		{"alt percent", NewFloat(0.25), "#.0%", "25.%", ""},
		// 3.14 fractional grouping, probed: format(1234.567891, '.6,f').
		{"frac comma", NewFloat(1234.567891), ".6,f", "1234.567,891", ""},
		{"frac underscore", NewFloat(1234.567891), ".6_f", "1234.567_891", ""},
		{"frac both parts", NewFloat(1234.567891), ",.6,f", "1,234.567,891", ""},
		{"frac mixed seps", NewFloat(1234.567891), ",.6_f", "1,234.567_891", ""},
		{"frac partial group", NewFloat(1234.56789), ".4,f", "1234.567,9", ""},
		{"frac short", NewFloat(1234.5), ".1,f", "1234.5", ""},
		{"frac e", NewFloat(1234.567891), ".6,e", "1.234,568e+03", ""},
		{"frac percent", NewFloat(0.123456), ".6,%", "12.345,600%", ""},
		{"frac none", NewFloat(0.123456789), ".4,", "0.123,5", ""},
		{"frac zero pad", NewFloat(1234.5678), "020,.6,f", "0,000,001,234.567,800", ""},
		{"n code", NewFloat(1234567.891), "n", "1.23457e+06", ""},

		{"unknown d", NewFloat(3.14), "d", "", "ValueError: Unknown format code 'd' for object of type 'float'"},
		{"unknown c", NewFloat(3.14), "c", "", "ValueError: Unknown format code 'c' for object of type 'float'"},
		{"unknown q", NewFloat(3.14), "q", "", "ValueError: Unknown format code 'q' for object of type 'float'"},
		{"unknown s", NewFloat(3.14), "+s", "", "ValueError: Unknown format code 's' for object of type 'float'"},
		{"invalid", NewFloat(3.14), "q q", "", "ValueError: Invalid format specifier 'q q' for object of type 'float'"},
		{"comma c", NewFloat(3.14), ",c", "", "ValueError: Cannot specify ',' with 'c'."},
		{"missing precision", NewFloat(1.5), ".f", "", "ValueError: Format specifier missing precision"},
		{"frac both error", NewFloat(1234.567891), ".6,_f", "", "ValueError: Cannot specify both ',' and '_'."},
		{"frac both reversed", NewFloat(1234.567891), ".6_,f", "", "ValueError: Cannot specify both ',' and '_'."},
		{"frac double comma", NewFloat(1234.567891), ".6,,f", "", "ValueError: Invalid format specifier '.6,,f' for object of type 'float'"},
	})
}

func TestFormatBool(t *testing.T) {
	runFmtCases(t, []fmtCase{
		// Probed on 3.14: bool falls to int's __format__ once the spec is
		// non-empty, but str(True) on an empty one.
		{"empty spec true", True, "", "True", ""},
		{"empty spec false", False, "", "False", ""},
		{"d", True, "d", "1", ""},
		{"width", True, ">6", "     1", ""},
		{"bare align", True, "<", "1", ""},
		{"plus", True, "+", "+1", ""},
		{"comma", True, ",", "1", ""},
		{"float code", True, ".2f", "1.00", ""},
		{"hex", True, "x", "1", ""},
		{"c", True, "c", "\x01", ""},
		{"alt hex false", False, "#x", "0x0", ""},
		{"zero pad false", False, "05", "00000", ""},
		// Errors name bool, not int. Probed on 3.14.
		{"unknown s", True, "s", "", "ValueError: Unknown format code 's' for object of type 'bool'"},
		{"invalid", True, "q q", "", "ValueError: Invalid format specifier 'q q' for object of type 'bool'"},
		{"z flag", True, "z", "", "ValueError: Negative zero coercion (z) not allowed in integer format specifier"},
	})
}

func TestFormatObject(t *testing.T) {
	runFmtCases(t, []fmtCase{
		// object.__format__: empty spec is str(o), anything else raises.
		// Probed on 3.14 per type name.
		{"none empty", None, "", "None", ""},
		{"list empty", L(NewInt(1), NewInt(2)), "", "[1, 2]", ""},
		{"exception empty", NewException(ValueError, []Object{NewStr("boom")}), "", "boom", ""},
		{"none s", None, "s", "", "TypeError: unsupported format string passed to NoneType.__format__"},
		{"none width", None, ">5", "", "TypeError: unsupported format string passed to NoneType.__format__"},
		{"list d", L(NewInt(1)), "d", "", "TypeError: unsupported format string passed to list.__format__"},
		{"tuple width", T(NewInt(1)), ">3", "", "TypeError: unsupported format string passed to tuple.__format__"},
		{"dict x", D(t, NewStr("a"), NewInt(1)), "x", "", "TypeError: unsupported format string passed to dict.__format__"},
		{"range width", NewRange(0, 3, 1), "5", "", "TypeError: unsupported format string passed to range.__format__"},
		{"exception width", NewException(ValueError, []Object{NewStr("boom")}), ">9", "", "TypeError: unsupported format string passed to ValueError.__format__"},
	})
}
