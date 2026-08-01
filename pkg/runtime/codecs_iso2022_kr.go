package runtime

// iso2022_kr is the Korean member of the ISO-2022 family and does not use the G0
// escape-designation model the JP codecs do. It designates KSC 5601 into G1 once
// with ESC$)C and then shifts between ascii and KSC 5601 with the SO (0x0e) and SI
// (0x0f) control bytes: bytes read after SO are KSC 5601 GL pairs, bytes read after
// SI are ascii. A newline (0x0a, and only 0x0a, not tab or CR) resets the shift back
// to ascii on decode, while the G1 designation persists. Every non-ascii character
// therefore routes through KSC 5601, so the encode map is the full KSC 5601
// repertoire (iso2022KRKSCEncode); the decode direction reuses the KSC 5601 decode
// table from codecs_iso2022_jp2_tables.go, whose GL pairs are identical.
//
// The engine mode packs the G1 designation code in the low byte (0x42 undesignated,
// 0xC3 KSC 5601) and the shift state (SO active) in bit 8. CPython's getstate lays
// the encoder state out as 0x4200 | G1<<16 | shift<<40 and the decoder state as
// 0x420042 | G1<<8 | shift<<32, which the state hooks below reproduce.

// iso2022KREncStateValue/iso2022KREncStateMode and their decoder counterparts pack
// the iso2022_kr shift-state the way CPython's getstate does.
func iso2022KREncStateValue(mode int) int64 {
	return 0x4200 | int64(mode&0xFF)<<16 | int64((mode>>8)&0x1)<<40
}
func iso2022KREncStateMode(v int64) int {
	return int((v>>16)&0xFF) | int((v>>40)&0x1)<<8
}
func iso2022KRDecStateValue(mode int) int64 {
	return 0x420042 | int64(mode&0xFF)<<8 | int64((mode>>8)&0x1)<<32
}
func iso2022KRDecStateMode(v int64) int {
	return int((v>>8)&0xFF) | int((v>>32)&0x1)<<8
}

// iso2022KRCodec is the engine codec iso2022_kr drives. The ground mode leaves G1
// undesignated (0x42) and the shift inactive, matching CPython's initial state.
var iso2022KRCodec = &mbCodec{
	name:     "iso2022_kr",
	initMode: iso2022ModeASCII,
	encodeStateful: func(runes []rune, errors string, final bool, mode int) ([]byte, []rune, int, error) {
		return iso2022KREncodeRun(runes, errors, final, mode)
	},
	decodeStateful: func(data []byte, errors string, final bool, mode int) (string, int, []byte, int, error) {
		return iso2022KRDecodeRun(data, errors, final, mode)
	},
	encStateValue: iso2022KREncStateValue,
	encStateMode:  iso2022KREncStateMode,
	decStateValue: iso2022KRDecStateValue,
	decStateMode:  iso2022KRDecStateMode,
}

// iso2022KREncodeRun encodes runes for iso2022_kr. An ascii code point is emitted
// after returning to the ascii shift with SI if a KSC run was open; a KSC 5601
// character designates G1 with ESC$)C the first time one appears, shifts in with SO
// if not already shifted, and emits its GL pair. The shift is left open across a
// non-final chunk and closed with SI before any ascii byte and at the end of a final
// chunk, the way CPython's iso2022_kr encoder does. A code point KSC 5601 cannot
// represent routes through the error handler. iso2022_kr holds no rune pending.
func iso2022KREncodeRun(runes []rune, errors string, final bool, mode int) ([]byte, []rune, int, error) {
	var out []byte
	g1 := byte(mode & 0xFF)
	shifted := (mode>>8)&0x1 == 1
	toASCII := func() {
		if shifted {
			out = append(out, 0x0f)
			shifted = false
		}
	}
	pack := func() int {
		m := int(g1)
		if shifted {
			m |= 1 << 8
		}
		return m
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < 0x80 {
			toASCII()
			out = append(out, byte(r))
			continue
		}
		if v, ok := iso2022KRKSCEncode[r]; ok {
			if g1 != iso2022ModeKSC5601 {
				out = append(out, 0x1b, '$', ')', 'C')
				g1 = iso2022ModeKSC5601
			}
			if !shifted {
				out = append(out, 0x0e)
				shifted = true
			}
			out = append(out, byte(v>>8), byte(v))
			continue
		}
		switch errors {
		case "strict":
			return nil, nil, 0, mbUnicodeEncodeError("iso2022_kr", r, i, "illegal multibyte sequence")
		case "ignore":
			// drop the code point, designation and shift unchanged
		case "replace":
			toASCII()
			out = append(out, '?')
		default:
			rep, err := mbEncodeHandler("iso2022_kr", runes, i, errors)
			if err != nil {
				return nil, nil, 0, err
			}
			out = append(out, rep...)
		}
	}
	if final {
		toASCII()
	}
	return out, nil, pack(), nil
}

// iso2022KRDecodeRun decodes bytes for iso2022_kr. ESC$)C designates KSC 5601 into
// G1; an ESC that does not start that sequence is either an illegal escape attempt
// (ESC$ or ESC( with the wrong following bytes) or, for any other second byte, a
// plain control byte passed through. SO (0x0e) and SI (0x0f) toggle the shift, a
// newline (0x0a) emits '\n' and resets the shift, and any other byte below 0x21 is a
// control passed through. In the ascii shift a byte 0x21..0x7f is ascii and a byte
// 0x80 or above is illegal one byte wide; in the KSC shift a byte 0x21..0x7f is the
// lead of a GL pair and a byte 0x80 or above is illegal one byte wide. A bad pair is
// illegal two bytes wide, an escape with a bad final byte is illegal over its whole
// width, and a truncated escape or a lone pair lead is incomplete (buffered when not
// final), matching CPython's iso2022_kr decoder.
func iso2022KRDecodeRun(data []byte, errors string, final bool, mode int) (string, int, []byte, int, error) {
	var out []rune
	i := 0
	g1 := byte(mode & 0xFF)
	shifted := (mode>>8)&0x1 == 1
	pack := func() int {
		m := int(g1)
		if shifted {
			m |= 1 << 8
		}
		return m
	}
	fail := func(start, end int, reason string) (int, error) {
		rep, np, err := mbDecodeError("iso2022_kr", data, start, end, reason, errors)
		if err != nil {
			return 0, err
		}
		out = append(out, rep...)
		return np, nil
	}
	buffer := func(i int) (string, int, []byte, int, error, bool) {
		if !final {
			return string(out), i, append([]byte(nil), data[i:]...), pack(), nil, true
		}
		return "", 0, nil, 0, nil, false
	}
	for i < len(data) {
		c := data[i]
		if c == 0x1b {
			if i+1 >= len(data) {
				if s, ci, buf, m, err, ok := buffer(i); ok {
					return s, ci, buf, m, err
				}
				np, err := fail(i, i+1, "incomplete multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			c1 := data[i+1]
			if c1 == '$' {
				if i+2 >= len(data) {
					if s, ci, buf, m, err, ok := buffer(i); ok {
						return s, ci, buf, m, err
					}
					np, err := fail(i, len(data), "incomplete multibyte sequence")
					if err != nil {
						return "", 0, nil, 0, err
					}
					i = np
					continue
				}
				if data[i+2] == ')' {
					if i+3 >= len(data) {
						if s, ci, buf, m, err, ok := buffer(i); ok {
							return s, ci, buf, m, err
						}
						np, err := fail(i, len(data), "incomplete multibyte sequence")
						if err != nil {
							return "", 0, nil, 0, err
						}
						i = np
						continue
					}
					if data[i+3] == 'C' {
						g1 = iso2022ModeKSC5601
						i += 4
						continue
					}
					np, err := fail(i, i+4, "illegal multibyte sequence")
					if err != nil {
						return "", 0, nil, 0, err
					}
					i = np
					continue
				}
				np, err := fail(i, i+3, "illegal multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			if c1 == '(' {
				if i+2 >= len(data) {
					if s, ci, buf, m, err, ok := buffer(i); ok {
						return s, ci, buf, m, err
					}
					np, err := fail(i, len(data), "incomplete multibyte sequence")
					if err != nil {
						return "", 0, nil, 0, err
					}
					i = np
					continue
				}
				np, err := fail(i, i+3, "illegal multibyte sequence")
				if err != nil {
					return "", 0, nil, 0, err
				}
				i = np
				continue
			}
			// ESC is a plain control byte here; emit it and reprocess c1.
			out = append(out, 0x1b)
			i++
			continue
		}
		if c == 0x0e {
			shifted = true
			i++
			continue
		}
		if c == 0x0f {
			shifted = false
			i++
			continue
		}
		if c == 0x0a {
			out = append(out, '\n')
			shifted = false
			i++
			continue
		}
		if c < 0x21 {
			out = append(out, rune(c))
			i++
			continue
		}
		if c >= 0x80 {
			np, err := fail(i, i+1, "illegal multibyte sequence")
			if err != nil {
				return "", 0, nil, 0, err
			}
			i = np
			continue
		}
		if !shifted {
			out = append(out, rune(c))
			i++
			continue
		}
		// KSC shift: c is a pair lead in 0x21..0x7f.
		if i+1 >= len(data) {
			if s, ci, buf, m, err, ok := buffer(i); ok {
				return s, ci, buf, m, err
			}
			np, err := fail(i, i+1, "incomplete multibyte sequence")
			if err != nil {
				return "", 0, nil, 0, err
			}
			i = np
			continue
		}
		key := uint16(c)<<8 | uint16(data[i+1])
		if cp, ok := iso2022KSC5601Decode[key]; ok {
			out = append(out, cp)
			i += 2
			continue
		}
		np, err := fail(i, i+2, "illegal multibyte sequence")
		if err != nil {
			return "", 0, nil, 0, err
		}
		i = np
	}
	return string(out), i, nil, pack(), nil
}
