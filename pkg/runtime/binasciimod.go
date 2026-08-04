package runtime

import (
	"bytes"
	"fmt"
	"hash/crc32"

	"github.com/tamnd/unagi/pkg/objects"
)

// binascii is the C accelerator the base64, uu and quopri modules convert
// between binary and the ASCII encodings on. base64.py imports it directly and
// has no pure fallback, so `import base64` needs binascii to exist as a Go
// builtin. This slice implements the base64 and hex codecs base64 drives, plus
// the two CRC helpers, the uuencode line codecs (b2a_uu/a2b_uu) that gate the
// uu module, and the Error and Incomplete exceptions. The quoted-printable
// codecs (b2a_qp/a2b_qp) are a later slice.

// binasciiErrorClass is binascii.Error, a subclass of ValueError that the
// codecs raise and base64 catches. It is built once and captured by the module
// closures.
var binasciiErrorClass objects.Object

func init() {
	moduleTable["binascii"] = &moduleEntry{builtin: true, exec: initBinascii}
}

func initBinascii(m *objects.Module) error {
	valueError, ok := objects.ExcClassValue("ValueError")
	if !ok {
		return objects.Raise(objects.RuntimeError, "binascii: ValueError base is unavailable")
	}
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "binascii: Exception base is unavailable")
	}
	errCls, err := objects.NewClass("Error", "binascii.Error", []objects.Object{valueError}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	binasciiErrorClass = errCls
	incomplete, err := objects.NewClass("Incomplete", "binascii.Incomplete", []objects.Object{exc}, nil, nil, nil, nil)
	if err != nil {
		return err
	}

	entries := []struct {
		name string
		val  objects.Object
	}{
		{"Error", errCls},
		{"Incomplete", incomplete},
		{"hexlify", objects.NewFunc("hexlify", -1, binasciiHexlify)},
		{"b2a_hex", objects.NewFunc("b2a_hex", -1, binasciiHexlify)},
		{"unhexlify", objects.NewFunc("unhexlify", 1, binasciiUnhexlify)},
		{"a2b_hex", objects.NewFunc("a2b_hex", 1, binasciiUnhexlify)},
		{"b2a_base64", objects.NewFuncKw("b2a_base64", binasciiB2aBase64)},
		{"a2b_base64", objects.NewFuncKw("a2b_base64", binasciiA2bBase64)},
		{"b2a_uu", objects.NewFuncKw("b2a_uu", binasciiB2aUu)},
		{"a2b_uu", objects.NewFunc("a2b_uu", 1, binasciiA2bUu)},
		{"crc32", objects.NewFunc("crc32", -1, binasciiCRC32)},
		{"crc_hqx", objects.NewFunc("crc_hqx", 2, binasciiCRCHqx)},
		{"a2b_qp", objects.NewFuncKw("a2b_qp", binasciiA2bQp)},
		{"b2a_qp", objects.NewFuncKw("b2a_qp", binasciiB2aQp)},
	}
	for _, e := range entries {
		if err := objects.StoreAttr(m, e.name, e.val); err != nil {
			return err
		}
	}
	return nil
}

// binasciiErrorf raises a binascii.Error carrying the formatted message.
func binasciiErrorf(format string, a ...any) error {
	inst, err := objects.Call(binasciiErrorClass, []objects.Object{objects.NewStr(fmt.Sprintf(format, a...))})
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return objects.Raise("Error", "%s", format)
}

// binasciiData reads a buffer or ASCII str argument, the way the C codecs
// accept any read-only buffer (bytes, bytearray, memoryview, array) or, for the
// a2b functions, an ASCII string.
func binasciiData(o objects.Object) ([]byte, error) {
	if b, ok := objects.AsBufferBytes(o); ok {
		return b, nil
	}
	if s, ok := objects.AsStr(o); ok {
		return []byte(s), nil
	}
	return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", o.TypeName())
}

const hexDigits = "0123456789abcdef"

// binasciiHexlify implements hexlify/b2a_hex: the lowercase hex of the data,
// with an optional single-byte separator inserted every bytes_per_sep bytes
// (counted from the right for a positive count, the left for a negative one).
func binasciiHexlify(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 {
		return nil, objects.Raise(objects.TypeError, "hexlify() takes at least 1 argument (0 given)")
	}
	data, err := binasciiData(args[0])
	if err != nil {
		return nil, err
	}
	if len(args) < 2 {
		out := make([]byte, 0, len(data)*2)
		for _, c := range data {
			out = append(out, hexDigits[c>>4], hexDigits[c&0x0f])
		}
		return objects.NewBytes(out), nil
	}
	sep, ok := objects.AsBytesLike(args[1])
	if !ok {
		if s, sok := objects.AsStr(args[1]); sok {
			sep = []byte(s)
		} else {
			return nil, objects.Raise(objects.TypeError, "sep must be str or bytes")
		}
	}
	if len(sep) != 1 {
		return nil, objects.Raise(objects.ValueError, "sep must be length 1")
	}
	bps := 1
	if len(args) >= 3 {
		v, iok := objects.AsInt(args[2])
		if !iok {
			return nil, objects.Raise(objects.TypeError, "an integer is required")
		}
		bps = int(v)
	}
	return objects.NewBytes(hexWithSep(data, sep[0], bps)), nil
}

// hexWithSep renders data as hex with a separator every abs(bps) bytes, grouped
// from the right when bps is positive and from the left when negative.
func hexWithSep(data []byte, sep byte, bps int) []byte {
	if bps == 0 || len(data) == 0 {
		out := make([]byte, 0, len(data)*2)
		for _, c := range data {
			out = append(out, hexDigits[c>>4], hexDigits[c&0x0f])
		}
		return out
	}
	fromRight := bps > 0
	group := bps
	if group < 0 {
		group = -group
	}
	var out []byte
	for i, c := range data {
		var pos int
		if fromRight {
			pos = len(data) - i
		} else {
			pos = i
		}
		if i > 0 && ((fromRight && pos%group == 0) || (!fromRight && i%group == 0)) {
			out = append(out, sep)
		}
		out = append(out, hexDigits[c>>4], hexDigits[c&0x0f])
	}
	return out
}

// binasciiUnhexlify implements unhexlify/a2b_hex: the bytes of a hex string,
// raising Error on an odd length or a non-hex digit.
func binasciiUnhexlify(args []objects.Object) (objects.Object, error) {
	data, err := binasciiData(args[0])
	if err != nil {
		return nil, err
	}
	if len(data)%2 != 0 {
		return nil, binasciiErrorf("Odd-length string")
	}
	out := make([]byte, len(data)/2)
	for i := range out {
		hi, ok1 := hexVal(data[2*i])
		lo, ok2 := hexVal(data[2*i+1])
		if !ok1 || !ok2 {
			return nil, binasciiErrorf("Non-hexadecimal digit found")
		}
		out[i] = hi<<4 | lo
	}
	return objects.NewBytes(out), nil
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

const base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

var base64Reverse = buildBase64Reverse()

func buildBase64Reverse() [256]byte {
	var t [256]byte
	for i := range t {
		t[i] = 0xff
	}
	for i := range len(base64Alphabet) {
		t[base64Alphabet[i]] = byte(i)
	}
	return t
}

// binasciiB2aBase64 implements b2a_base64(data, *, newline=True): the base64 of
// the data on one line, with a trailing newline unless newline is false.
func binasciiB2aBase64(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "b2a_base64() takes exactly 1 positional argument (%d given)", len(pos))
	}
	newline := true
	for i, k := range kwNames {
		if k != "newline" {
			return nil, objects.Raise(objects.TypeError, "b2a_base64() got an unexpected keyword argument '%s'", k)
		}
		newline = objects.Truth(kwVals[i])
	}
	data, err := binasciiData(pos[0])
	if err != nil {
		return nil, err
	}
	var out []byte
	leftbits := 0
	leftchar := 0
	for _, b := range data {
		leftchar = (leftchar << 8) | int(b)
		leftbits += 8
		for leftbits >= 6 {
			this := (leftchar >> (leftbits - 6)) & 0x3f
			leftbits -= 6
			out = append(out, base64Alphabet[this])
		}
	}
	switch leftbits {
	case 2:
		out = append(out, base64Alphabet[(leftchar&3)<<4], '=', '=')
	case 4:
		out = append(out, base64Alphabet[(leftchar&0xf)<<2], '=')
	}
	if newline {
		out = append(out, '\n')
	}
	return objects.NewBytes(out), nil
}

// binasciiA2bBase64 implements a2b_base64(data, *, strict_mode=False), a direct
// port of the CPython state machine: non-strict silently skips non-alphabet
// bytes, while strict rejects them and the several padding faults with their
// own messages.
func binasciiA2bBase64(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "a2b_base64() takes exactly 1 positional argument (%d given)", len(pos))
	}
	strict := false
	for i, k := range kwNames {
		if k != "strict_mode" {
			return nil, objects.Raise(objects.TypeError, "a2b_base64() got an unexpected keyword argument '%s'", k)
		}
		strict = objects.Truth(kwVals[i])
	}
	data, err := binasciiData(pos[0])
	if err != nil {
		return nil, err
	}
	var out []byte
	quadPos := 0
	var leftchar byte
	pads := 0
	for i, ch := range data {
		if ch == '=' {
			pads++
			if quadPos >= 2 && quadPos+pads <= 4 {
				continue
			}
			if !strict {
				continue
			}
			if quadPos == 1 {
				break
			}
			if quadPos == 0 && i == 0 {
				return nil, binasciiErrorf("Leading padding not allowed")
			}
			return nil, binasciiErrorf("Excess padding not allowed")
		}
		v := base64Reverse[ch]
		if v >= 64 {
			if strict {
				return nil, binasciiErrorf("Only base64 data is allowed")
			}
			continue
		}
		if pads != 0 && strict {
			if quadPos+pads == 4 {
				return nil, binasciiErrorf("Excess data after padding")
			}
			return nil, binasciiErrorf("Discontinuous padding not allowed")
		}
		pads = 0
		switch quadPos {
		case 0:
			quadPos = 1
			leftchar = v
		case 1:
			quadPos = 2
			out = append(out, leftchar<<2|v>>4)
			leftchar = v & 0x0f
		case 2:
			quadPos = 3
			out = append(out, leftchar<<4|v>>2)
			leftchar = v & 0x03
		case 3:
			quadPos = 0
			out = append(out, leftchar<<6|v)
			leftchar = 0
		}
	}
	if quadPos == 1 {
		return nil, binasciiErrorf("Invalid base64-encoded string: number of data characters (%d) cannot be 1 more than a multiple of 4",
			len(out)/3*4+1)
	}
	if quadPos != 0 && quadPos+pads < 4 {
		return nil, binasciiErrorf("Incorrect padding")
	}
	return objects.NewBytes(out), nil
}

// binasciiB2aUu implements b2a_uu(data, *, backtick=False): one uuencoded line
// for at most 45 input bytes. The first character carries the length, each
// 6-bit group maps to value+0x20, and a zero group becomes 0x60 (backtick) when
// backtick is set instead of a space. A courtesy newline ends the line.
func binasciiB2aUu(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "b2a_uu() takes exactly 1 positional argument (%d given)", len(pos))
	}
	backtick := false
	for i, k := range kwNames {
		if k != "backtick" {
			return nil, objects.Raise(objects.TypeError, "b2a_uu() got an unexpected keyword argument '%s'", k)
		}
		backtick = objects.Truth(kwVals[i])
	}
	data, err := binasciiData(pos[0])
	if err != nil {
		return nil, err
	}
	if len(data) > 45 {
		return nil, binasciiErrorf("At most 45 bytes at once")
	}
	var out []byte
	if backtick && len(data) == 0 {
		out = append(out, 0x60)
	} else {
		out = append(out, ' '+byte(len(data)&0x3f))
	}
	leftchar := 0
	leftbits := 0
	for i := 0; i < len(data) || leftbits != 0; i++ {
		if i < len(data) {
			leftchar = leftchar<<8 | int(data[i])
		} else {
			leftchar <<= 8
		}
		leftbits += 8
		for leftbits >= 6 {
			this := (leftchar >> (leftbits - 6)) & 0x3f
			leftbits -= 6
			if backtick && this == 0 {
				out = append(out, 0x60)
			} else {
				out = append(out, byte(this)+' ')
			}
		}
	}
	out = append(out, '\n')
	return objects.NewBytes(out), nil
}

// binasciiA2bUu implements a2b_uu(data): decode one uuencoded line. The first
// character gives the byte length, the rest are 6-bit groups (space or backtick
// meaning zero), and any characters past the length must be whitespace-only.
func binasciiA2bUu(args []objects.Object) (objects.Object, error) {
	data, err := binasciiData(args[0])
	if err != nil {
		return nil, err
	}
	binLen := 0
	p := 0
	if len(data) > 0 {
		binLen = int(data[0]-' ') & 0x3f
		p = 1
	}
	out := make([]byte, 0, binLen)
	leftchar := 0
	leftbits := 0
	for len(out) < binLen {
		var this int
		if p >= len(data) {
			this = 0
		} else {
			ch := data[p]
			if ch == '\n' || ch == '\r' {
				this = 0
			} else if ch < ' ' || ch > ' '+64 {
				return nil, binasciiErrorf("Illegal char")
			} else {
				this = int(ch-' ') & 0x3f
			}
			p++
		}
		leftchar = leftchar<<6 | this
		leftbits += 6
		if leftbits >= 8 {
			leftbits -= 8
			out = append(out, byte((leftchar>>leftbits)&0xff))
			leftchar &= (1 << leftbits) - 1
		}
	}
	for ; p < len(data); p++ {
		ch := data[p]
		if ch != ' ' && ch != ' '+64 && ch != '\n' && ch != '\r' {
			return nil, binasciiErrorf("Trailing garbage")
		}
	}
	return objects.NewBytes(out), nil
}

// binasciiCRC32 implements crc32(data, value=0) with the standard IEEE
// polynomial, returning an unsigned 32-bit result.
func binasciiCRC32(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 {
		return nil, objects.Raise(objects.TypeError, "crc32() takes at least 1 argument (0 given)")
	}
	data, err := binasciiData(args[0])
	if err != nil {
		return nil, err
	}
	var seed uint32
	if len(args) >= 2 {
		v, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "an integer is required")
		}
		seed = uint32(v)
	}
	return objects.NewInt(int64(crc32.Update(seed, crc32.IEEETable, data))), nil
}

// binasciiCRCHqx implements crc_hqx(data, value): the CRC-CCITT (XModem) 16-bit
// checksum, MSB first with polynomial 0x1021.
func binasciiCRCHqx(args []objects.Object) (objects.Object, error) {
	data, err := binasciiData(args[0])
	if err != nil {
		return nil, err
	}
	v, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	crc := uint16(v)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return objects.NewInt(int64(crc)), nil
}

// qpMaxLineSize is CPython's MAXLINESIZE, the column at which b2a_qp inserts a
// soft line break.
const qpMaxLineSize = 76

// qpToHex renders a byte as the two uppercase hex digits an =XX escape carries,
// CPython's to_hex.
func qpToHex(ch byte) (byte, byte) {
	const hex = "0123456789ABCDEF"
	return hex[ch>>4], hex[ch&0x0f]
}

// qpIsHexDigit reports whether ch is one of 0-9, A-F or a-f, the bytes an
// =XX escape may carry.
func qpIsHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')
}

// qpHexValue maps a hex digit byte to its numeric value; the callers only pass
// bytes qpIsHexDigit already accepted.
func qpHexValue(ch byte) byte {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0'
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10
	default:
		return ch - 'A' + 10
	}
}

// binasciiA2bQp implements a2b_qp(data, header=False): decode a quoted-printable
// block, a direct port of CPython's binascii_a2b_qp_impl. An =XX pair decodes to
// the byte, an = before a newline is a soft break that drops the newline, and
// with header true an underscore decodes to a space.
func binasciiA2bQp(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 || len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError, "a2b_qp() takes at most 2 arguments (%d given)", len(pos))
	}
	header := false
	if len(pos) == 2 {
		header = objects.Truth(pos[1])
	}
	for i, k := range kwNames {
		switch k {
		case "header":
			header = objects.Truth(kwVals[i])
		default:
			return nil, objects.Raise(objects.TypeError, "a2b_qp() got an unexpected keyword argument '%s'", k)
		}
	}
	data, err := binasciiData(pos[0])
	if err != nil {
		return nil, err
	}
	n := len(data)
	var out []byte
	in := 0
	for in < n {
		switch {
		case data[in] == '=':
			in++
			if in >= n {
				in = n
				break
			}
			if data[in] == '\n' || data[in] == '\r' {
				if data[in] != '\n' {
					for in < n && data[in] != '\n' {
						in++
					}
				}
				if in < n {
					in++
				}
			} else if data[in] == '=' {
				out = append(out, '=')
				in++
			} else if in+1 < n && qpIsHexDigit(data[in]) && qpIsHexDigit(data[in+1]) {
				ch := qpHexValue(data[in]) << 4
				in++
				ch |= qpHexValue(data[in])
				in++
				out = append(out, ch)
			} else {
				out = append(out, '=')
			}
		case header && data[in] == '_':
			out = append(out, ' ')
			in++
		default:
			out = append(out, data[in])
			in++
		}
	}
	return objects.NewBytes(out), nil
}

// qpNeedsQuote reports whether the byte at in must be emitted as an =XX escape,
// the shared predicate of CPython's binascii_b2a_qp_impl (identical in its
// measuring and writing passes).
func qpNeedsQuote(data []byte, in, n int, header, istext, quotetabs bool, linelen int) bool {
	c := data[in]
	if c > 126 || c == '=' {
		return true
	}
	if header && c == '_' {
		return true
	}
	if c == '.' && linelen == 0 &&
		(in+1 == n || data[in+1] == '\n' || data[in+1] == '\r' || data[in+1] == 0) {
		return true
	}
	if !istext && (c == '\r' || c == '\n') {
		return true
	}
	if (c == '\t' || c == ' ') && in+1 == n {
		return true
	}
	if c < 33 && c != '\r' && c != '\n' && (quotetabs || (c != '\t' && c != ' ')) {
		return true
	}
	return false
}

// binasciiB2aQp implements b2a_qp(data, quotetabs=False, istext=True,
// header=False): encode a block as quoted-printable, a direct port of CPython's
// binascii_b2a_qp_impl. Bytes outside the printable range (plus '=' and the
// context-sensitive whitespace and leading dot) become =XX escapes, soft line
// breaks keep lines under 76 columns, and the CRLF form is mirrored when the
// input already uses it.
func binasciiB2aQp(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 || len(pos) > 4 {
		return nil, objects.Raise(objects.TypeError, "b2a_qp() takes at most 4 arguments (%d given)", len(pos))
	}
	quotetabs, istext, header := false, true, false
	if len(pos) >= 2 {
		quotetabs = objects.Truth(pos[1])
	}
	if len(pos) >= 3 {
		istext = objects.Truth(pos[2])
	}
	if len(pos) >= 4 {
		header = objects.Truth(pos[3])
	}
	for i, k := range kwNames {
		switch k {
		case "quotetabs":
			quotetabs = objects.Truth(kwVals[i])
		case "istext":
			istext = objects.Truth(kwVals[i])
		case "header":
			header = objects.Truth(kwVals[i])
		default:
			return nil, objects.Raise(objects.TypeError, "b2a_qp() got an unexpected keyword argument '%s'", k)
		}
	}
	data, err := binasciiData(pos[0])
	if err != nil {
		return nil, err
	}
	n := len(data)
	// crlf mirrors CPython's memchr probe: the first newline preceded by a
	// carriage return marks the input as CRLF, so the soft breaks match its form.
	crlf := false
	if idx := bytes.IndexByte(data, '\n'); idx > 0 && data[idx-1] == '\r' {
		crlf = true
	}
	var out []byte
	linelen := 0
	in := 0
	for in < n {
		if qpNeedsQuote(data, in, n, header, istext, quotetabs, linelen) {
			if linelen+3 >= qpMaxLineSize {
				out = append(out, '=')
				if crlf {
					out = append(out, '\r')
				}
				out = append(out, '\n')
				linelen = 0
			}
			h1, h2 := qpToHex(data[in])
			out = append(out, '=', h1, h2)
			in++
			linelen += 3
			continue
		}
		if istext && (data[in] == '\n' || (in+1 < n && data[in] == '\r' && data[in+1] == '\n')) {
			linelen = 0
			// A space or tab immediately before a hard newline is quoted so it
			// survives transport, CPython rewriting the already-emitted byte.
			if len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t') {
				ch := out[len(out)-1]
				out[len(out)-1] = '='
				h1, h2 := qpToHex(ch)
				out = append(out, h1, h2)
			}
			if crlf {
				out = append(out, '\r')
			}
			out = append(out, '\n')
			if data[in] == '\r' {
				in += 2
			} else {
				in++
			}
			continue
		}
		if in+1 != n && data[in+1] != '\n' && linelen+1 >= qpMaxLineSize {
			out = append(out, '=')
			if crlf {
				out = append(out, '\r')
			}
			out = append(out, '\n')
			linelen = 0
		}
		linelen++
		if header && data[in] == ' ' {
			out = append(out, '_')
			in++
		} else {
			out = append(out, data[in])
			in++
		}
	}
	return objects.NewBytes(out), nil
}
