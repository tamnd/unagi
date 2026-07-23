package runtime

import (
	"fmt"
	"hash/crc32"

	"github.com/tamnd/unagi/pkg/objects"
)

// binascii is the C accelerator the base64, uu and quopri modules convert
// between binary and the ASCII encodings on. base64.py imports it directly and
// has no pure fallback, so `import base64` needs binascii to exist as a Go
// builtin. This slice implements the base64 and hex codecs base64 drives, plus
// the two CRC helpers, and the Error and Incomplete exceptions. The
// quoted-printable and uuencode codecs (b2a_qp/a2b_qp, b2a_uu/a2b_uu) are a
// later slice; they gate quopri and uu, not base64.

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
		{"crc32", objects.NewFunc("crc32", -1, binasciiCRC32)},
		{"crc_hqx", objects.NewFunc("crc_hqx", 2, binasciiCRCHqx)},
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

// binasciiData reads a bytes-like or ASCII str argument, the way the C codecs
// accept a read-only buffer or (for the a2b functions) an ASCII string.
func binasciiData(o objects.Object) ([]byte, error) {
	if b, ok := objects.AsBytesLike(o); ok {
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
