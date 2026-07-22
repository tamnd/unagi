package objects

import "strings"

// StructSeqType is a C structseq class, the tuple subclass os.stat_result and
// the other os result types use. It differs from a namedtuple in two ways a
// namedType cannot express: it carries named fields that are NOT part of the
// sequence (st_atime_ns, st_blksize, ...), and a named field can hold a value
// different from the tuple slot at the same index (st[7] is the int seconds,
// st.st_atime is the float). The type keeps the field metadata; each instance
// is a tupleObject whose elts are the visible sequence and whose sseq binding
// holds the full named-value vector.
type StructSeqType struct {
	name     string   // the type name, e.g. "stat_result"
	reprName string   // the repr prefix, e.g. "os.stat_result"
	fields   []string // every named field, in repr order
	nSeq     int      // n_sequence_fields: the visible tuple length
	nUnnamed int      // n_unnamed_fields
}

func (*StructSeqType) TypeName() string { return "type" }

// structSeqBinding is the per-instance link back to the type plus the named
// values, aligned to typ.fields. The first typ.nSeq fields are the named view
// of the sequence; the rest are named-only. Values here are the attribute
// forms (float times), which is why they are kept apart from the tuple elts.
type structSeqBinding struct {
	typ  *StructSeqType
	vals []Object
}

// NewStructSeqType builds a structseq class. fields lists every named field in
// repr order; nSeq is how many entries the sequence exposes; nUnnamed is the
// structseq n_unnamed_fields count. It is small and immutable, shared by every
// instance.
func NewStructSeqType(name, reprName string, fields []string, nSeq, nUnnamed int) *StructSeqType {
	return &StructSeqType{name: name, reprName: reprName, fields: fields, nSeq: nSeq, nUnnamed: nUnnamed}
}

// NewStructSeq builds one instance. seq is the visible sequence (len nSeq) and
// named is the full named-value vector (len len(fields)). The two overlap for
// the first fields but may differ where a slot carries a different attribute
// form, as the stat time fields do.
func (t *StructSeqType) NewStructSeq(seq, named []Object) Object {
	return &tupleObject{elts: seq, sseq: &structSeqBinding{typ: t, vals: named}}
}

// structSeqTypeAttr resolves an attribute read on the class object: the name
// and the structseq field-count descriptors.
func structSeqTypeAttr(t *StructSeqType, name string) (Object, error) {
	switch name {
	case "__name__", "__qualname__":
		return NewStr(t.name), nil
	case "n_fields":
		return NewInt(int64(len(t.fields))), nil
	case "n_sequence_fields":
		return NewInt(int64(t.nSeq)), nil
	case "n_unnamed_fields":
		return NewInt(int64(t.nUnnamed)), nil
	}
	return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", t.name, name)
}

// structSeqInstanceAttr resolves an attribute read on an instance: a named
// field by name, or the field-count descriptors the type also answers.
func structSeqInstanceAttr(x *tupleObject, name string) (Object, error) {
	b := x.sseq
	for i, f := range b.typ.fields {
		if f == name {
			return b.vals[i], nil
		}
	}
	switch name {
	case "n_fields":
		return NewInt(int64(len(b.typ.fields))), nil
	case "n_sequence_fields":
		return NewInt(int64(b.typ.nSeq)), nil
	case "n_unnamed_fields":
		return NewInt(int64(b.typ.nUnnamed)), nil
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", b.typ.name, name)
}

// structSeqRepr spells reprName(field=value, ...) over every named field,
// matching CPython's structseq repr.
func structSeqRepr(x *tupleObject, strict bool) (string, error) {
	b := x.sseq
	var s strings.Builder
	s.WriteString(b.typ.reprName)
	s.WriteByte('(')
	for i, f := range b.typ.fields {
		if i > 0 {
			s.WriteString(", ")
		}
		v, err := reprCore(b.vals[i], strict)
		if err != nil {
			return "", err
		}
		s.WriteString(f)
		s.WriteByte('=')
		s.WriteString(v)
	}
	s.WriteByte(')')
	return s.String(), nil
}
