package objects

import "testing"

// floatBaseValue is the float builtin as a class statement names it: a funcObject
// spelled "float" that converts its first argument, the conversion a value
// subclass runs to build its payload. builtinBaseName keys off the name.
func floatBaseValue() Object {
	return NewFunc("float", -1, func(args []Object) (Object, error) {
		if len(args) == 0 {
			return NewFloat(0), nil
		}
		v, ok := AsFloat(args[0])
		if !ok {
			return nil, Raise(TypeError, "float() argument must be a number")
		}
		return NewFloat(v), nil
	})
}

// complexBaseValue is the complex builtin as a class statement names it, building
// the complex payload a value subclass carries from up to two numeric arguments.
func complexBaseValue() Object {
	return NewFunc("complex", -1, func(args []Object) (Object, error) {
		var re, im Object
		if len(args) > 0 {
			re = args[0]
		}
		if len(args) > 1 {
			im = args[1]
		}
		return ComplexNew(re, im)
	})
}

func buildFloatSubclass(t *testing.T, name string) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{floatBaseValue()}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc := c.(*classObject)
	if cc.builtinBase != "float" {
		t.Fatalf("build %s: builtinBase = %q, want float", name, cc.builtinBase)
	}
	return cc
}

func buildComplexSubclass(t *testing.T, name string) *classObject {
	t.Helper()
	c, err := buildClass(nil, name, "__main__."+name, []Object{complexBaseValue()}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build %s: %v", name, err)
	}
	cc := c.(*classObject)
	if cc.builtinBase != "complex" {
		t.Fatalf("build %s: builtinBase = %q, want complex", name, cc.builtinBase)
	}
	return cc
}

func TestFloatSubclassArithmeticReturnsPlainFloat(t *testing.T) {
	c := buildFloatSubclass(t, "MyFloat")
	a, err := Instantiate(c, []Object{NewFloat(2.5)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyFloat(2.5): %v", err)
	}

	sum, err := Add(a, NewFloat(0.5))
	if err != nil || Str(sum) != "3.0" {
		t.Fatalf("a + 0.5 = %v, %v; want 3.0", sum, err)
	}
	if _, isInst := sum.(*instanceObject); isInst {
		t.Fatalf("a + 0.5 kept the subclass; want a plain float")
	}
	if sum.TypeName() != "float" {
		t.Fatalf("type(a + 0.5) = %q, want float", sum.TypeName())
	}

	// A float subclass reads as its stored double for math coercion, the way
	// PyFloat_AsDouble does, so AsFloat must unwrap the payload.
	if d, ok := AsFloat(a); !ok || d != 2.5 {
		t.Fatalf("AsFloat(MyFloat(2.5)) = %v, %v; want 2.5, true", d, ok)
	}
}

func TestFloatSubclassInheritsMethods(t *testing.T) {
	c := buildFloatSubclass(t, "MyFloat")
	a, err := Instantiate(c, []Object{NewFloat(4.0)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"is_integer", "as_integer_ratio", "conjugate", "hex", "__getnewargs__"} {
		if _, err := LoadAttr(a, name); err != nil {
			t.Fatalf("MyFloat(4.0).%s: %v", name, err)
		}
	}
	// real and imag resolve as data attributes off the payload.
	re, err := LoadAttr(a, "real")
	if err != nil || Str(re) != "4.0" {
		t.Fatalf("MyFloat(4.0).real = %v, %v; want 4.0", re, err)
	}
	isInt, err := CallMethod(a, "is_integer", nil)
	if err != nil || isInt != True {
		t.Fatalf("MyFloat(4.0).is_integer() = %v, %v; want True", isInt, err)
	}
}

func TestFloatSubclassReducesToNewargs(t *testing.T) {
	c := buildFloatSubclass(t, "MyFloat")
	a, err := Instantiate(c, []Object{NewFloat(2.5)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	red, err := objectReduceNewobj(a)
	if err != nil {
		t.Fatalf("reduce: %v", err)
	}
	tup, ok := red.(*tupleObject)
	if !ok || len(tup.elts) != 3 {
		t.Fatalf("reduce shape = %v; want a 3-tuple", red)
	}
	// The new-args carry the class then the payload double, so object.__new__(cls,
	// 2.5) rebuilds the subclass to its stored value rather than a blank 0.0.
	newArgs, ok := tup.elts[1].(*tupleObject)
	if !ok || len(newArgs.elts) != 2 {
		t.Fatalf("newargs = %v; want (cls, 2.5)", tup.elts[1])
	}
	if Str(newArgs.elts[1]) != "2.5" {
		t.Fatalf("newargs[1] = %v; want 2.5", newArgs.elts[1])
	}
}

func TestComplexSubclassCoercesAndReduces(t *testing.T) {
	c := buildComplexSubclass(t, "MyComplex")
	z, err := Instantiate(c, []Object{NewFloat(3), NewFloat(4)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyComplex(3, 4): %v", err)
	}

	// ComplexParts unwraps a complex subclass so cmath and complex() read its
	// stored real and imaginary parts.
	re, im, ok := ComplexParts(z)
	if !ok || re != 3 || im != 4 {
		t.Fatalf("ComplexParts(MyComplex(3,4)) = %v, %v, %v; want 3, 4, true", re, im, ok)
	}

	sum, err := Add(z, NewInt(1))
	if err != nil || sum.TypeName() != "complex" {
		t.Fatalf("z + 1 = %v (%s), %v; want a plain complex", sum, sum.TypeName(), err)
	}

	red, err := objectReduceNewobj(z)
	if err != nil {
		t.Fatalf("reduce: %v", err)
	}
	newArgs := red.(*tupleObject).elts[1].(*tupleObject)
	if len(newArgs.elts) != 3 {
		t.Fatalf("newargs = %v; want (cls, 3.0, 4.0)", newArgs)
	}
	if Str(newArgs.elts[1]) != "3.0" || Str(newArgs.elts[2]) != "4.0" {
		t.Fatalf("newargs = %v; want real 3.0 and imag 4.0", newArgs.elts)
	}
}
