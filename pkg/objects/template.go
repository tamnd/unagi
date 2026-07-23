package objects

import "fmt"

// templateObject is a string.templatelib.Template, the value a PEP 750 t-string
// evaluates to. It holds the static string parts and the Interpolation objects
// interleaved between them; strings always number one more than interps, with
// empty strings filling the gaps, so strings[i] precedes interps[i] and the
// last string trails the final interpolation.
//
// The bare type name is Template so type(t).__name__ reads the way CPython
// spells it; the module-qualified name appears only in attribute-error wording.
type templateObject struct {
	strings []Object
	interps []Object
}

func (*templateObject) TypeName() string { return "Template" }

// NewTemplate builds a Template from the static string parts and the
// interpolations between them.
func NewTemplate(strings, interps []Object) Object {
	return &templateObject{strings: strings, interps: interps}
}

// interpolationObject is a string.templatelib.Interpolation: one field of a
// t-string. value is the evaluated expression, expression its verbatim source,
// conversion the None-or-str conversion character, and formatSpec the evaluated
// format spec. PEP 750 leaves applying the conversion and spec to the consumer,
// so they are recorded, not applied.
type interpolationObject struct {
	value      Object
	expression string
	conversion Object
	formatSpec Object
}

func (*interpolationObject) TypeName() string { return "Interpolation" }

// NewInterpolation builds one t-string field. formatSpec is a str object, since
// a spec with a nested field is evaluated at runtime rather than fixed.
func NewInterpolation(value Object, expression string, conversion, formatSpec Object) Object {
	return &interpolationObject{value: value, expression: expression, conversion: conversion, formatSpec: formatSpec}
}

// templateLoadAttr answers the attributes a Template exposes: the static parts,
// the interpolations, and the shortcut tuple of interpolation values.
func templateLoadAttr(t *templateObject, name string) (Object, error) {
	switch name {
	case "strings":
		return NewTuple(append([]Object(nil), t.strings...)), nil
	case "interpolations":
		return NewTuple(append([]Object(nil), t.interps...)), nil
	case "values":
		vals := make([]Object, len(t.interps))
		for i, in := range t.interps {
			vals[i] = in.(*interpolationObject).value
		}
		return NewTuple(vals), nil
	}
	return nil, Raise(AttributeError, "'string.templatelib.Template' object has no attribute '%s'", name)
}

// templateIter interleaves the static parts and interpolations in source order,
// dropping empty strings the way iterating a Template does in CPython.
func (t *templateObject) templateIter() []Object {
	out := make([]Object, 0, len(t.strings)+len(t.interps))
	for i, s := range t.strings {
		if str, ok := s.(*strObject); !ok || str.v != "" {
			out = append(out, s)
		}
		if i < len(t.interps) {
			out = append(out, t.interps[i])
		}
	}
	return out
}

func templateRepr(t *templateObject) (string, error) {
	ss, err := ReprE(NewTuple(append([]Object(nil), t.strings...)))
	if err != nil {
		return "", err
	}
	is, err := ReprE(NewTuple(append([]Object(nil), t.interps...)))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Template(strings=%s, interpolations=%s)", ss, is), nil
}

// interpolationLoadAttr answers the four attributes an Interpolation exposes.
func interpolationLoadAttr(in *interpolationObject, name string) (Object, error) {
	switch name {
	case "value":
		return in.value, nil
	case "expression":
		return NewStr(in.expression), nil
	case "conversion":
		return in.conversion, nil
	case "format_spec":
		return in.formatSpec, nil
	}
	return nil, Raise(AttributeError, "'string.templatelib.Interpolation' object has no attribute '%s'", name)
}

func interpolationRepr(in *interpolationObject) (string, error) {
	vr, err := ReprE(in.value)
	if err != nil {
		return "", err
	}
	cr, err := ReprE(in.conversion)
	if err != nil {
		return "", err
	}
	er, err := ReprE(NewStr(in.expression))
	if err != nil {
		return "", err
	}
	fr, err := ReprE(in.formatSpec)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Interpolation(%s, %s, %s, %s)", vr, er, cr, fr), nil
}
