package runtime

import (
	"fmt"
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// _multibytecodec is a C accelerator module in CPython, the engine behind every
// CJK multibyte codec (the _codecs_cn/_jp/_kr/_tw/_hk/_iso2022 families). It has
// no pure fallback, so it has to exist in Go before any of those encodings can
// load. This slice brings up the engine and drives it with the first real codec,
// gb2312 (registered from _codecs_cn), so the whole path from `import codecs` to
// a byte-exact roundtrip runs end to end.
//
// The engine is the stateless encode/decode the codec object exposes plus the
// incremental encoder/decoder and stream reader/writer the encodings.<name>
// modules subclass. A concrete codec is an mbCodec: a name and the two per-unit
// step functions that say how one code point encodes and how the next bytes
// decode. The engine loops over those steps and applies the error handler the
// same way CPython's multibytecodec_encerror/decerror do: strict raises a real
// UnicodeEncodeError/UnicodeDecodeError, ignore drops the unit, replace emits
// the replacement, and any other handler routes through codecs.lookup_error
// (whose non-strict handlers still raise NotImplementedError in this tier).
//
// The Python-visible types are real classes built with objects.NewClass so the
// vendored encodings/gb2312.py can subclass them by multiple inheritance
// alongside the codecs module's Codec/IncrementalEncoder/StreamReader bases and
// set `codec = _codecs_cn.getcodec('gb2312')` as a class attribute. The engine
// reads that codec back off self at call time.

// mbStatus values are the outcome of encoding or decoding one unit.
const (
	mbOK      = iota // a code point encoded, or bytes decoded, cleanly
	mbTooFew         // decode: the trailing bytes are an incomplete sequence
	mbIllegal        // this unit cannot be mapped
)

// mbCodec is one concrete multibyte codec. encodeStep encodes a single code
// point, returning its bytes and mbOK, or mbIllegal when the code point has no
// mapping. decodeStep decodes the next unit from p (always at least one byte),
// returning the code point and how many bytes it consumed on mbOK, mbTooFew when
// p holds only the start of a longer sequence, or mbIllegal with esize set to
// how many bytes the bad sequence spans. cp2 is a second code point for the JIS
// X 0213 sequences that decode to a base plus a combining mark, or -1 when the
// unit decodes to a single code point (the common case).
//
// encodePair and encodeBase are set only by the combining codecs: encodePair is
// tried before encodeStep and encodes a two-code-point sequence, returning
// ok=false when the pair is not a combined one; encodeBase reports whether a code
// point can begin such a pair, so the incremental encoder holds it pending across
// a chunk boundary. Both are nil for the plain codecs, and the engine skips the
// lookahead then.
type mbCodec struct {
	name       string
	encodeStep func(cp rune) ([]byte, int)
	decodeStep func(p []byte) (cp rune, cp2 rune, consumed int, esize int, status int)
	encodePair func(a, b rune) ([]byte, bool)
	encodeBase func(r rune) bool
}

// mbTableCodec is a table-driven multibyte codec: some bytes decode on their
// own (ascii and, for the JP/KR codecs, half-width katakana), a lead byte begins
// a two-byte character looked up in the double map, and everything else is
// illegal. The encode maps are built from the oracle encoder so a code point
// with more than one decoding still encodes to the canonical bytes. It covers
// the fixed-width double-byte codecs (shift_jis and its relatives); the codecs
// with three-byte or shift-state sequences carry their own step functions.
type mbTableCodec struct {
	name      string
	single    map[byte]rune
	lead      [256]bool
	double    map[uint16]rune
	multi     map[uint16][2]rune // byte pair -> two code points, the JIS X 0213 combining decodes
	encSingle map[rune]byte
	encDouble map[rune]uint16
	encPair   map[[2]rune]uint16 // (base, mark) -> two bytes
	base      map[rune]bool      // code points that can begin a combining pair
}

// newMBTableCodec assembles a table-driven codec from the generated maps and the
// lead-byte list.
func newMBTableCodec(name string, single map[byte]rune, leads []byte, double map[uint16]rune, encSingle map[rune]byte, encDouble map[rune]uint16) *mbTableCodec {
	tc := &mbTableCodec{name: name, single: single, double: double, encSingle: encSingle, encDouble: encDouble}
	for _, b := range leads {
		tc.lead[b] = true
	}
	return tc
}

// withCombining attaches the JIS X 0213 combining tables to a table codec: the
// two-byte sequences that decode to a base plus a mark, the encode map for those
// pairs, and the set of code points that can begin one. It is what turns a plain
// shift_jis-shaped codec into shift_jis_2004.
func (tc *mbTableCodec) withCombining(multi map[uint16][2]rune, encPair map[[2]rune]uint16, bases []rune) *mbTableCodec {
	tc.multi = multi
	tc.encPair = encPair
	tc.base = make(map[rune]bool, len(bases))
	for _, r := range bases {
		tc.base[r] = true
	}
	return tc
}

// codec adapts the table-driven codec to the engine's mbCodec interface. The
// combining hooks are wired only when the codec carries a combining table, so a
// plain codec leaves them nil and the engine skips the pair lookahead.
func (tc *mbTableCodec) codec() *mbCodec {
	c := &mbCodec{name: tc.name, encodeStep: tc.encodeStep, decodeStep: tc.decodeStep}
	if len(tc.multi) > 0 {
		c.encodePair = tc.encodePairStep
		c.encodeBase = tc.encodeBaseStep
	}
	return c
}

// decodeStep decodes the next unit: a standalone byte, or a two-byte character
// after a lead byte. A lead byte with nothing after it is incomplete; a lead
// with an unmapped trail, or any non-lead non-standalone byte, is illegal and
// spans one byte, matching CPython's decoders for this family. A two-byte
// sequence in the combining table decodes to a base plus a mark (cp2 >= 0).
func (tc *mbTableCodec) decodeStep(p []byte) (rune, rune, int, int, int) {
	c := p[0]
	if r, ok := tc.single[c]; ok {
		return r, -1, 1, 0, mbOK
	}
	if tc.lead[c] {
		if len(p) < 2 {
			return 0, -1, 0, 0, mbTooFew
		}
		key := uint16(c)<<8 | uint16(p[1])
		if r, ok := tc.double[key]; ok {
			return r, -1, 2, 0, mbOK
		}
		if m, ok := tc.multi[key]; ok {
			return m[0], m[1], 2, 0, mbOK
		}
		return 0, -1, 0, 1, mbIllegal
	}
	return 0, -1, 0, 1, mbIllegal
}

// encodePairStep encodes a two-code-point combining sequence, or reports that
// the pair is not a combined one.
func (tc *mbTableCodec) encodePairStep(a, b rune) ([]byte, bool) {
	if v, ok := tc.encPair[[2]rune{a, b}]; ok {
		return []byte{byte(v >> 8), byte(v)}, true
	}
	return nil, false
}

// encodeBaseStep reports whether a code point can begin a combining pair.
func (tc *mbTableCodec) encodeBaseStep(r rune) bool { return tc.base[r] }

// encodeStep encodes one code point through the standalone-byte and two-byte
// encode maps, or reports it unmappable.
func (tc *mbTableCodec) encodeStep(cp rune) ([]byte, int) {
	if b, ok := tc.encSingle[cp]; ok {
		return []byte{b}, mbOK
	}
	if v, ok := tc.encDouble[cp]; ok {
		return []byte{byte(v >> 8), byte(v)}, mbOK
	}
	return nil, mbIllegal
}

// mbEUCJPCodec is the table-driven codec for euc_jp, a variable-width EUC codec
// the fixed-width mbTableCodec cannot express. It decodes ascii on its own, 0x8e
// (SS2) followed by one byte as a half-width katakana, 0x8f (SS3) followed by two
// bytes as a JIS X 0212 character, and any other high byte as the lead of a
// two-byte JIS X 0208 character. Every high byte begins a multibyte sequence, so
// a lone high byte is incomplete and a bad trail is illegal one byte wide,
// matching CPython. The euc_jis_2004 variants share this byte structure and can
// reuse it once the engine also carries their multi-character decodes.
type mbEUCJPCodec struct {
	name      string
	lead      [256]bool
	double    map[uint16]rune // lead<<8|trail: 0x8e katakana and the JIS X 0208 plane
	triple    map[uint16]rune // the two bytes after 0x8f: b2<<8|b3
	encDouble map[rune]uint16
	encTriple map[rune]uint16
}

// eucJPSS3 is the single-shift byte that introduces a three-byte JIS X 0212
// sequence; 0x8e introduces a two-byte katakana and is handled as a plain lead.
const eucJPSS3 = 0x8F

// newMBEUCJPCodec assembles the euc_jp codec from the generated maps and the
// lead-byte list.
func newMBEUCJPCodec(name string, leads []byte, double, triple map[uint16]rune, encDouble, encTriple map[rune]uint16) *mbEUCJPCodec {
	ec := &mbEUCJPCodec{name: name, double: double, triple: triple, encDouble: encDouble, encTriple: encTriple}
	for _, b := range leads {
		ec.lead[b] = true
	}
	return ec
}

// codec adapts the euc_jp codec to the engine's mbCodec interface.
func (ec *mbEUCJPCodec) codec() *mbCodec {
	return &mbCodec{name: ec.name, encodeStep: ec.encodeStep, decodeStep: ec.decodeStep}
}

// decodeStep decodes the next unit. ascii decodes on its own; a lead needs two
// bytes, or three when it is the 0x8f single-shift; too few bytes for the width
// is incomplete and reported over the bytes in hand, and a lead whose trail does
// not map is illegal and spans just the lead byte, so the decoder resyncs one
// byte on and reports the following bytes separately, matching CPython.
func (ec *mbEUCJPCodec) decodeStep(p []byte) (rune, rune, int, int, int) {
	c := p[0]
	if c < 0x80 {
		return rune(c), -1, 1, 0, mbOK
	}
	if !ec.lead[c] {
		return 0, -1, 0, 1, mbIllegal
	}
	width := 2
	if c == eucJPSS3 {
		width = 3
	}
	if len(p) < width {
		return 0, -1, 0, 0, mbTooFew
	}
	if width == 3 {
		if r, ok := ec.triple[uint16(p[1])<<8|uint16(p[2])]; ok {
			return r, -1, 3, 0, mbOK
		}
		return 0, -1, 0, 1, mbIllegal
	}
	if r, ok := ec.double[uint16(c)<<8|uint16(p[1])]; ok {
		return r, -1, 2, 0, mbOK
	}
	return 0, -1, 0, 1, mbIllegal
}

// encodeStep encodes one code point: ascii as itself, a two-byte JIS X 0208 or
// 0x8e katakana value straight, and a JIS X 0212 value behind the 0x8f
// single-shift, or reports it unmappable.
func (ec *mbEUCJPCodec) encodeStep(cp rune) ([]byte, int) {
	if cp < 0x80 {
		return []byte{byte(cp)}, mbOK
	}
	if v, ok := ec.encDouble[cp]; ok {
		return []byte{byte(v >> 8), byte(v)}, mbOK
	}
	if v, ok := ec.encTriple[cp]; ok {
		return []byte{eucJPSS3, byte(v >> 8), byte(v)}, mbOK
	}
	return nil, mbIllegal
}

// mbCodecCarrier smuggles a Go *mbCodec onto a Python MultibyteCodec instance as
// a hidden attribute. The Object surface is only the type name; the engine reads
// the pointer back off it and never exposes it to Python.
type mbCodecCarrier struct{ c *mbCodec }

func (*mbCodecCarrier) TypeName() string { return "_multibytecodec.codec" }

// mbCarrierAttr is the slot the codec pointer lives in on a MultibyteCodec
// instance. The double-underscore name keeps it out of the way of the public
// encode/decode surface.
const mbCarrierAttr = "__mbcodec__"

// The Python-visible classes, built once so their identity is stable across
// imports: isinstance and the subclass MROs in encodings.<name> depend on the
// same class objects every import returning the same value.
var (
	mbBuildOnce         sync.Once
	mbBuildErr          error
	mbCodecClass        objects.Object
	mbIncEncoderClass   objects.Object
	mbIncDecoderClass   objects.Object
	mbStreamReaderClass objects.Object
	mbStreamWriterClass objects.Object
)

func init() {
	moduleTable["_multibytecodec"] = &moduleEntry{builtin: true, exec: initMultibytecodec}
}

// initMultibytecodec binds the engine's Python-visible classes on the module.
// _codecs_cn.getcodec builds the codec object directly in Go, so the C module's
// __create_codec capsule entry point is not needed here.
func initMultibytecodec(m *objects.Module) error {
	if err := buildMBClasses(); err != nil {
		return err
	}
	surface := []struct {
		name string
		cls  objects.Object
	}{
		{"MultibyteIncrementalEncoder", mbIncEncoderClass},
		{"MultibyteIncrementalDecoder", mbIncDecoderClass},
		{"MultibyteStreamReader", mbStreamReaderClass},
		{"MultibyteStreamWriter", mbStreamWriterClass},
	}
	for _, s := range surface {
		if err := objects.StoreAttr(m, s.name, s.cls); err != nil {
			return err
		}
	}
	return nil
}

// buildMBClasses constructs the engine's classes on first use. MultibyteCodec is
// the stateless codec object _codecs_cn.getcodec hands back; the incremental and
// stream classes are the bases encodings.<name> subclasses, reading `self.codec`
// (set by the subclass) to reach the engine.
func buildMBClasses() error {
	mbBuildOnce.Do(func() {
		mbCodecClass, mbBuildErr = objects.NewClass("MultibyteCodec", "_multibytecodec.MultibyteCodec", nil,
			[]string{"encode", "decode"},
			[]objects.Object{
				objects.NewMethodKw("encode", mbCodecEncode),
				objects.NewMethodKw("decode", mbCodecDecode),
			}, nil, nil)
		if mbBuildErr != nil {
			return
		}
		mbIncEncoderClass, mbBuildErr = objects.NewClass("MultibyteIncrementalEncoder", "_multibytecodec.MultibyteIncrementalEncoder", nil,
			[]string{"__init__", "encode", "reset", "getstate", "setstate"},
			[]objects.Object{
				objects.NewMethodKw("__init__", mbIncInit),
				objects.NewMethodKw("encode", mbIncEncode),
				objects.NewMethod("reset", 1, mbIncEncReset),
				objects.NewMethod("getstate", 1, mbIncEncGetstate),
				objects.NewMethod("setstate", 2, mbIncEncSetstate),
			}, nil, nil)
		if mbBuildErr != nil {
			return
		}
		mbIncDecoderClass, mbBuildErr = objects.NewClass("MultibyteIncrementalDecoder", "_multibytecodec.MultibyteIncrementalDecoder", nil,
			[]string{"__init__", "decode", "reset", "getstate", "setstate"},
			[]objects.Object{
				objects.NewMethodKw("__init__", mbIncInit),
				objects.NewMethodKw("decode", mbIncDecode),
				objects.NewMethod("reset", 1, mbIncDecReset),
				objects.NewMethod("getstate", 1, mbIncDecGetstate),
				objects.NewMethod("setstate", 2, mbIncDecSetstate),
			}, nil, nil)
		if mbBuildErr != nil {
			return
		}
		// The stream reader and writer carry no engine logic of their own: the
		// codecs module's StreamReader/StreamWriter bases drive read and write
		// through the Codec.encode/decode the subclass mixes in, so these exist
		// only as the multibyte marker in the subclass MRO.
		mbStreamReaderClass, mbBuildErr = objects.NewClass("MultibyteStreamReader", "_multibytecodec.MultibyteStreamReader", nil, nil, nil, nil, nil)
		if mbBuildErr != nil {
			return
		}
		mbStreamWriterClass, mbBuildErr = objects.NewClass("MultibyteStreamWriter", "_multibytecodec.MultibyteStreamWriter", nil, nil, nil, nil, nil)
	})
	return mbBuildErr
}

// newMultibyteCodec builds a MultibyteCodec instance carrying the given engine
// codec, the object _codecs_cn.getcodec returns.
func newMultibyteCodec(c *mbCodec) (objects.Object, error) {
	if err := buildMBClasses(); err != nil {
		return nil, err
	}
	inst, err := objects.Call(mbCodecClass, nil)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(inst, mbCarrierAttr, &mbCodecCarrier{c: c}); err != nil {
		return nil, err
	}
	return inst, nil
}

// mbCodecOf reads the engine codec off a MultibyteCodec instance.
func mbCodecOf(self objects.Object) (*mbCodec, error) {
	v, err := objects.LoadAttr(self, mbCarrierAttr)
	if err != nil {
		return nil, err
	}
	car, ok := v.(*mbCodecCarrier)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "not a MultibyteCodec")
	}
	return car.c, nil
}

// mbCodecOfSelf reads the engine codec off `self.codec`, the class attribute the
// encodings.<name> subclass sets to the MultibyteCodec getcodec returned.
func mbCodecOfSelf(self objects.Object) (*mbCodec, error) {
	codecObj, err := objects.LoadAttr(self, "codec")
	if err != nil {
		return nil, err
	}
	return mbCodecOf(codecObj)
}

// mbCodecEncode is MultibyteCodec.encode(input, errors='strict'): the stateless
// encode the codecs module's Codec.encode is bound to. It returns (bytes,
// length) where length is the number of input code points.
func mbCodecEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self, s, errors, err := mbCodecStrCall("encode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	c, err := mbCodecOf(self)
	if err != nil {
		return nil, err
	}
	runes := objects.StrRunes(s)
	out, _, err := mbEncodeRun(c, runes, errors, true)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewBytes(out), objects.NewInt(int64(len(runes)))}), nil
}

// mbCodecDecode is MultibyteCodec.decode(input, errors='strict'): the stateless
// decode the codecs module's Codec.decode is bound to. It returns (str, length)
// where length is the number of input bytes consumed.
func mbCodecDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self, errors, data, err := mbCodecBytesCall("decode", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	c, err := mbCodecOf(self)
	if err != nil {
		return nil, err
	}
	out, consumed, _, err := mbDecodeRun(c, data, errors, true)
	if err != nil {
		return nil, err
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(out), objects.NewInt(int64(consumed))}), nil
}

// mbIncInit is MultibyteIncrementalEncoder/Decoder __init__(self,
// errors='strict'): it records the error handler name and clears the decoder's
// pending-byte buffer. The encoder keeps no pending state for the double-byte
// codecs, so the buffer is harmless there.
func mbIncInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "__init__() missing self")
	}
	self := pos[0]
	errors := "strict"
	if len(pos) >= 2 {
		e, ok := objects.AsStr(pos[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "errors must be str, not %s", pos[1].TypeName())
		}
		errors = e
	}
	for i, kn := range kwNames {
		if kn == "errors" {
			e, ok := objects.AsStr(kwVals[i])
			if !ok {
				return nil, objects.Raise(objects.TypeError, "errors must be str, not %s", kwVals[i].TypeName())
			}
			errors = e
		}
	}
	if err := objects.StoreAttr(self, "errors", objects.NewStr(errors)); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_buffer", objects.NewBytes(nil)); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_pendingchar", objects.NewStr("")); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// mbIncEncode is MultibyteIncrementalEncoder.encode(self, input, final=False):
// it prepends any combining base held from the last chunk, encodes as far as it
// can, and holds a trailing combining base back unless final is set. The plain
// double-byte codecs never hold a base (a lone surrogate is unmappable and
// errors at once, matching CPython), so final does not change their result.
func mbIncEncode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 2 {
		return nil, objects.Raise(objects.TypeError, "encode() missing required argument 'input'")
	}
	self := pos[0]
	s, ok := objects.AsStr(pos[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "encode() argument 'input' must be str, not %s", pos[1].TypeName())
	}
	final := false
	if len(pos) >= 3 {
		f, err := objects.TruthOf(pos[2])
		if err != nil {
			return nil, err
		}
		final = f
	}
	for i, kn := range kwNames {
		if kn == "final" {
			f, err := objects.TruthOf(kwVals[i])
			if err != nil {
				return nil, err
			}
			final = f
		}
	}
	c, err := mbCodecOfSelf(self)
	if err != nil {
		return nil, err
	}
	errors := mbErrorsOf(self)
	runes := append(append([]rune(nil), mbPendingOf(self)...), objects.StrRunes(s)...)
	out, pending, err := mbEncodeRun(c, runes, errors, final)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_pendingchar", objects.NewStr(string(pending))); err != nil {
		return nil, err
	}
	return objects.NewBytes(out), nil
}

// mbIncDecode is MultibyteIncrementalDecoder.decode(self, input, final=False):
// it prepends any buffered bytes, decodes as far as it can, and keeps a trailing
// incomplete sequence in the buffer unless final is set, in which case an
// incomplete tail is an error.
func mbIncDecode(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 2 {
		return nil, objects.Raise(objects.TypeError, "decode() missing required argument 'input'")
	}
	self := pos[0]
	v, ok := objects.AsBytesLike(pos[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "decode() argument 'input' must be bytes-like, not %s", pos[1].TypeName())
	}
	final := false
	if len(pos) >= 3 {
		f, err := objects.TruthOf(pos[2])
		if err != nil {
			return nil, err
		}
		final = f
	}
	for i, kn := range kwNames {
		if kn == "final" {
			f, err := objects.TruthOf(kwVals[i])
			if err != nil {
				return nil, err
			}
			final = f
		}
	}
	c, err := mbCodecOfSelf(self)
	if err != nil {
		return nil, err
	}
	errors := mbErrorsOf(self)
	data := append(append([]byte(nil), mbBufferOf(self)...), v...)
	out, _, pending, err := mbDecodeRun(c, data, errors, final)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, "_buffer", objects.NewBytes(pending)); err != nil {
		return nil, err
	}
	return objects.NewStr(out), nil
}

// mbIncEncReset / mbIncDecReset clear the incremental state. The encoder drops
// any combining base held pending; the decoder drops any buffered bytes.
func mbIncEncReset(args []objects.Object) (objects.Object, error) {
	if err := objects.StoreAttr(args[0], "_pendingchar", objects.NewStr("")); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func mbIncDecReset(args []objects.Object) (objects.Object, error) {
	if err := objects.StoreAttr(args[0], "_buffer", objects.NewBytes(nil)); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// mbIncEncGetstate / mbIncEncSetstate report and restore the encoder state. The
// double-byte codecs have no shift state and never buffer, so the state is
// always the integer 0, matching CPython's encoder_getstate for this family.
func mbIncEncGetstate(args []objects.Object) (objects.Object, error) { return objects.NewInt(0), nil }

func mbIncEncSetstate(args []objects.Object) (objects.Object, error) { return objects.None, nil }

// mbIncDecGetstate reports the decoder state as (pending_bytes, 0): the buffered
// incomplete bytes and the codec's shift state, which is always 0 here.
func mbIncDecGetstate(args []objects.Object) (objects.Object, error) {
	return objects.NewTuple([]objects.Object{objects.NewBytes(mbBufferOf(args[0])), objects.NewInt(0)}), nil
}

// mbIncDecSetstate restores the decoder state from a (pending_bytes, flags)
// tuple, keeping the pending bytes for the next decode.
func mbIncDecSetstate(args []objects.Object) (objects.Object, error) {
	self, state := args[0], args[1]
	first, err := objects.GetItem(state, objects.NewInt(0))
	if err != nil {
		return nil, err
	}
	b, ok := objects.AsBytesLike(first)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "setstate() argument must be a (bytes, int) tuple")
	}
	if err := objects.StoreAttr(self, "_buffer", objects.NewBytes(b)); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// mbErrorsOf reads the error handler name off an incremental instance, defaulting
// to strict when it has not been set.
func mbErrorsOf(self objects.Object) string {
	v, err := objects.LoadAttr(self, "errors")
	if err != nil {
		return "strict"
	}
	if s, ok := objects.AsStr(v); ok {
		return s
	}
	return "strict"
}

// mbBufferOf reads the decoder's pending-byte buffer.
func mbBufferOf(self objects.Object) []byte {
	v, err := objects.LoadAttr(self, "_buffer")
	if err != nil {
		return nil
	}
	b, _ := objects.AsBytesLike(v)
	return b
}

// mbPendingOf reads the encoder's held combining base, the runes carried over
// from the last non-final chunk. It is empty for the plain codecs.
func mbPendingOf(self objects.Object) []rune {
	v, err := objects.LoadAttr(self, "_pendingchar")
	if err != nil {
		return nil
	}
	if s, ok := objects.AsStr(v); ok {
		return objects.StrRunes(s)
	}
	return nil
}

// mbCodecStrCall reads the (self, input:str, errors='strict') shape the stateless
// encode takes.
func mbCodecStrCall(who string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (self objects.Object, s, errors string, err error) {
	errors = "strict"
	if len(pos) < 2 {
		return nil, "", "", objects.Raise(objects.TypeError, "%s() missing required argument 'input'", who)
	}
	self = pos[0]
	str, ok := objects.AsStr(pos[1])
	if !ok {
		return nil, "", "", objects.Raise(objects.TypeError, "%s() argument 'input' must be str, not %s", who, pos[1].TypeName())
	}
	s = str
	if len(pos) >= 3 && pos[2] != objects.None {
		e, ok := objects.AsStr(pos[2])
		if !ok {
			return nil, "", "", objects.Raise(objects.TypeError, "%s() argument 'errors' must be str, not %s", who, pos[2].TypeName())
		}
		errors = e
	}
	for i, kn := range kwNames {
		if kn == "errors" {
			e, ok := objects.AsStr(kwVals[i])
			if !ok {
				return nil, "", "", objects.Raise(objects.TypeError, "%s() argument 'errors' must be str, not %s", who, kwVals[i].TypeName())
			}
			errors = e
		}
	}
	return self, s, errors, nil
}

// mbCodecBytesCall reads the (self, input:bytes-like, errors='strict') shape the
// stateless decode takes.
func mbCodecBytesCall(who string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (self objects.Object, errors string, data []byte, err error) {
	errors = "strict"
	if len(pos) < 2 {
		return nil, "", nil, objects.Raise(objects.TypeError, "%s() missing required argument 'input'", who)
	}
	self = pos[0]
	v, ok := objects.AsBytesLike(pos[1])
	if !ok {
		return nil, "", nil, objects.Raise(objects.TypeError, "%s() argument 'input' must be bytes-like, not %s", who, pos[1].TypeName())
	}
	data = v
	if len(pos) >= 3 && pos[2] != objects.None {
		e, ok := objects.AsStr(pos[2])
		if !ok {
			return nil, "", nil, objects.Raise(objects.TypeError, "%s() argument 'errors' must be str, not %s", who, pos[2].TypeName())
		}
		errors = e
	}
	for i, kn := range kwNames {
		if kn == "errors" {
			e, ok := objects.AsStr(kwVals[i])
			if !ok {
				return nil, "", nil, objects.Raise(objects.TypeError, "%s() argument 'errors' must be str, not %s", who, kwVals[i].TypeName())
			}
			errors = e
		}
	}
	return self, errors, data, nil
}

// mbEncodeRun encodes runes through the codec, applying the error handler to any
// unmappable code point. It returns the encoded bytes and, when not final, any
// trailing combining base held back for the next chunk (empty for the plain
// codecs, which have no encoder state). A combining codec is encoded with a
// one-code-point lookahead: a base followed by its mark encodes as one unit, and
// a lone trailing base at the end of a non-final chunk is held pending.
func mbEncodeRun(c *mbCodec, runes []rune, errors string, final bool) ([]byte, []rune, error) {
	var out []byte
	i := 0
	for i < len(runes) {
		if !final && i == len(runes)-1 && c.encodeBase != nil && c.encodeBase(runes[i]) {
			break
		}
		if c.encodePair != nil && i+1 < len(runes) {
			if b, ok := c.encodePair(runes[i], runes[i+1]); ok {
				out = append(out, b...)
				i += 2
				continue
			}
		}
		b, status := c.encodeStep(runes[i])
		if status == mbOK {
			out = append(out, b...)
			i++
			continue
		}
		switch errors {
		case "strict":
			return nil, nil, mbUnicodeEncodeError(c.name, runes[i], i, "illegal multibyte sequence")
		case "ignore":
			// drop the code point
		case "replace":
			// CPython encodes the replacement '?' through the codec.
			rb, _ := c.encodeStep('?')
			out = append(out, rb...)
		default:
			rep, err := mbEncodeHandler(c.name, runes, i, errors)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, rep...)
		}
		i++
	}
	return out, append([]rune(nil), runes[i:]...), nil
}

// mbDecodeRun decodes data through the codec, applying the error handler to any
// illegal or (when final) incomplete sequence. It returns the decoded str, the
// number of bytes consumed, and, when not final, the trailing incomplete bytes
// held back for the next chunk.
func mbDecodeRun(c *mbCodec, data []byte, errors string, final bool) (string, int, []byte, error) {
	var out []rune
	i := 0
	for i < len(data) {
		cp, cp2, n, esize, status := c.decodeStep(data[i:])
		switch status {
		case mbOK:
			out = append(out, cp)
			if cp2 >= 0 {
				out = append(out, cp2)
			}
			i += n
		case mbTooFew:
			if !final {
				return string(out), i, append([]byte(nil), data[i:]...), nil
			}
			rep, newpos, err := mbDecodeError(c.name, data, i, len(data), "incomplete multibyte sequence", errors)
			if err != nil {
				return "", 0, nil, err
			}
			out = append(out, rep...)
			i = newpos
		case mbIllegal:
			rep, newpos, err := mbDecodeError(c.name, data, i, i+esize, "illegal multibyte sequence", errors)
			if err != nil {
				return "", 0, nil, err
			}
			out = append(out, rep...)
			i = newpos
		}
	}
	return string(out), i, nil, nil
}

// mbDecodeError applies the decode error handler to the span [start,end). strict
// raises, ignore skips the span, replace emits one U+FFFD, and any other handler
// routes through codecs.lookup_error. It returns the replacement runes and the
// position to resume at.
func mbDecodeError(codec string, data []byte, start, end int, reason, errors string) ([]rune, int, error) {
	switch errors {
	case "strict":
		return nil, 0, mbUnicodeDecodeError(codec, data, start, end, reason)
	case "ignore":
		return nil, end, nil
	case "replace":
		return []rune{0xFFFD}, end, nil
	default:
		return mbDecodeHandler(codec, data, start, end, reason, errors)
	}
}

// mbUnicodeEncodeError builds the UnicodeEncodeError strict raises for an
// unmappable code point, with CPython's wording.
func mbUnicodeEncodeError(codec string, r rune, pos int, reason string) error {
	return objects.Raise("UnicodeEncodeError",
		"'%s' codec can't encode character %s in position %d: %s",
		codec, mbCharEscape(r), pos, reason)
}

// mbUnicodeDecodeError builds the UnicodeDecodeError strict raises. A single bad
// byte reports "byte 0xNN in position P"; a wider span reports the range.
func mbUnicodeDecodeError(codec string, data []byte, start, end int, reason string) error {
	if end-start == 1 {
		return objects.Raise("UnicodeDecodeError",
			"'%s' codec can't decode byte 0x%02x in position %d: %s", codec, data[start], start, reason)
	}
	return objects.Raise("UnicodeDecodeError",
		"'%s' codec can't decode bytes in position %d-%d: %s", codec, start, end-1, reason)
}

// mbCharEscape renders a code point the way CPython's UnicodeEncodeError message
// does: '\xNN' below 0x100, '\uNNNN' in the BMP, '\U00NNNNNN' above it.
func mbCharEscape(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf(`'\x%02x'`, r)
	case r < 0x10000:
		return fmt.Sprintf(`'\u%04x'`, r)
	default:
		return fmt.Sprintf(`'\U%08x'`, r)
	}
}

// mbDecodeHandler routes a decode error to a registered handler through
// codecs.lookup_error, the path CPython's multibytecodec_decerror takes for a
// handler beyond strict/ignore/replace. The non-strict standard handlers raise
// NotImplementedError in this tier; a custom handler returns (replacement:str,
// newpos:int) and this resolves a negative newpos from the end and bounds-checks
// it the way CPython does.
func mbDecodeHandler(codec string, data []byte, start, end int, reason, errors string) ([]rune, int, error) {
	handler, err := codecLookupError([]objects.Object{objects.NewStr(errors)})
	if err != nil {
		return nil, 0, err
	}
	exc, err := mbAsException(mbUnicodeDecodeError(codec, data, start, end, reason))
	if err != nil {
		return nil, 0, err
	}
	res, err := objects.Call(handler, []objects.Object{exc})
	if err != nil {
		return nil, 0, err
	}
	rep, newpos, err := mbHandlerResult(res, len(data))
	if err != nil {
		return nil, 0, err
	}
	return objects.StrRunes(rep), newpos, nil
}

// mbEncodeHandler routes an encode error to a registered handler, the encode-side
// counterpart of mbDecodeHandler. It returns the bytes to emit for the span.
func mbEncodeHandler(codec string, runes []rune, pos int, errors string) ([]byte, error) {
	handler, err := codecLookupError([]objects.Object{objects.NewStr(errors)})
	if err != nil {
		return nil, err
	}
	exc, err := mbAsException(mbUnicodeEncodeError(codec, runes[pos], pos, "illegal multibyte sequence"))
	if err != nil {
		return nil, err
	}
	res, err := objects.Call(handler, []objects.Object{exc})
	if err != nil {
		return nil, err
	}
	rep, _, err := mbHandlerResult(res, len(runes))
	if err != nil {
		return nil, err
	}
	return []byte(rep), nil
}

// mbAsException adapts the error a Raise helper returns into the exception Object
// an error handler is called with.
func mbAsException(raised error) (objects.Object, error) {
	if o, ok := raised.(objects.Object); ok {
		return o, nil
	}
	return nil, raised
}

// mbHandlerResult reads the (replacement, newpos) tuple an error handler returns,
// resolving a negative newpos from length and bounds-checking it.
func mbHandlerResult(res objects.Object, length int) (string, int, error) {
	repObj, err := objects.GetItem(res, objects.NewInt(0))
	if err != nil {
		return "", 0, objects.Raise(objects.TypeError, "error handler must return a (str, int) tuple")
	}
	rep, ok := objects.AsStr(repObj)
	if !ok {
		return "", 0, objects.Raise(objects.TypeError, "error handler must return a (str, int) tuple")
	}
	posObj, err := objects.GetItem(res, objects.NewInt(1))
	if err != nil {
		return "", 0, objects.Raise(objects.TypeError, "error handler must return a (str, int) tuple")
	}
	newpos, ok := objects.AsInt(posObj)
	if !ok {
		return "", 0, objects.Raise(objects.TypeError, "error handler must return a (str, int) tuple")
	}
	pos := int(newpos)
	if pos < 0 {
		pos += length
	}
	if pos < 0 || pos > length {
		return "", 0, objects.Raise(objects.IndexError, "position %d from error handler out of bounds", newpos)
	}
	return rep, pos, nil
}
