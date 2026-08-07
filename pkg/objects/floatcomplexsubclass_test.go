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

// A float subclass that overrides an arithmetic dunder dispatches its own
// method instead of the native float fast path, on both operand orders, the way
// CPython calls the subclass slot rather than float's arithmetic. The reflected
// slot answers when the subclass sits on the right of a plain float.
func TestFloatSubclassOverrideDispatches(t *testing.T) {
	c := buildFloatSubclass(t, "MyFloat")
	c.setAttr("__mul__", mkfn("MyFloat.__mul__", 2, func(args []Object) (Object, error) {
		return NewStr("mul"), nil
	}))
	c.setAttr("__rmul__", mkfn("MyFloat.__rmul__", 2, func(args []Object) (Object, error) {
		return NewStr("rmul"), nil
	}))
	c.setAttr("__add__", mkfn("MyFloat.__add__", 2, func(args []Object) (Object, error) {
		return NewStr("add"), nil
	}))
	a, err := Instantiate(c, []Object{NewFloat(2.0)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyFloat(2.0): %v", err)
	}

	// Forward slot: the subclass on the left runs its own __mul__ and __add__.
	if r, err := Mul(a, NewInt(3)); err != nil || Str(r) != "mul" {
		t.Fatalf("MyFloat(2.0) * 3 = %v, %v; want 'mul'", r, err)
	}
	if r, err := Add(a, NewFloat(3.0)); err != nil || Str(r) != "add" {
		t.Fatalf("MyFloat(2.0) + 3.0 = %v, %v; want 'add'", r, err)
	}
	// Reflected slot: a plain float on the left defers to the subclass __rmul__.
	if r, err := Mul(NewFloat(3.0), a); err != nil || Str(r) != "rmul" {
		t.Fatalf("3.0 * MyFloat(2.0) = %v, %v; want 'rmul'", r, err)
	}
	if r, err := Mul(NewInt(3), a); err != nil || Str(r) != "rmul" {
		t.Fatalf("3 * MyFloat(2.0) = %v, %v; want 'rmul'", r, err)
	}
}

// A complex subclass that overrides an arithmetic dunder dispatches its own
// method instead of the native complex fast path, even when the other operand is
// itself complex, the case the fast path used to steal before it could reach the
// dunder protocol. The reflected slot answers when the subclass sits on the
// right of a plain complex, and a subclass that inherits the base arithmetic
// keeps returning a plain complex.
func TestComplexSubclassOverrideDispatches(t *testing.T) {
	c := buildComplexSubclass(t, "MyComplex")
	c.setAttr("__mul__", mkfn("MyComplex.__mul__", 2, func(args []Object) (Object, error) {
		return NewStr("mul"), nil
	}))
	c.setAttr("__rmul__", mkfn("MyComplex.__rmul__", 2, func(args []Object) (Object, error) {
		return NewStr("rmul"), nil
	}))
	c.setAttr("__sub__", mkfn("MyComplex.__sub__", 2, func(args []Object) (Object, error) {
		return NewStr("sub"), nil
	}))
	z, err := Instantiate(c, []Object{NewFloat(1), NewFloat(2)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyComplex(1, 2): %v", err)
	}

	// Forward slot with a complex operand: the subclass runs its own __sub__ and
	// __mul__ rather than the native complex arithmetic.
	if r, err := Sub(z, NewComplex(0, 1)); err != nil || Str(r) != "sub" {
		t.Fatalf("MyComplex(1+2j) - 1j = %v, %v; want 'sub'", r, err)
	}
	if r, err := Mul(z, NewComplex(0, 1)); err != nil || Str(r) != "mul" {
		t.Fatalf("MyComplex(1+2j) * 1j = %v, %v; want 'mul'", r, err)
	}
	// Reflected slot: a plain complex on the left defers to the subclass __rmul__.
	if r, err := Mul(NewComplex(0, 1), z); err != nil || Str(r) != "rmul" {
		t.Fatalf("1j * MyComplex(1+2j) = %v, %v; want 'rmul'", r, err)
	}

	// A subclass that inherits the base arithmetic keeps the native fast path and
	// returns a plain complex.
	plain := buildComplexSubclass(t, "Plain")
	p, err := Instantiate(plain, []Object{NewFloat(1), NewFloat(2)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate Plain(1, 2): %v", err)
	}
	if r, err := Sub(p, NewComplex(0, 1)); err != nil || r.TypeName() != "complex" {
		t.Fatalf("Plain(1+2j) - 1j = %v (%s), %v; want a plain complex", r, r.TypeName(), err)
	}
}

// A numeric subclass reaches its base arithmetic through super(): the dunder
// resolves off the unwrapped payload and returns the plain scalar, the way
// float.__truediv__ or complex.__mul__ answers on a bare value. A name the base
// does not carry stays unhandled so super() keeps its own miss and the object
// defaults still run for __init__.
func TestNumericSubclassSuperBaseDunder(t *testing.T) {
	fc := buildFloatSubclass(t, "MyFloat")
	f, err := Instantiate(fc, []Object{NewFloat(6.0)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyFloat(6.0): %v", err)
	}
	r, handled, err := builtinBaseCall(f, "__truediv__", []Object{NewFloat(2.0)}, nil, nil)
	if !handled || err != nil || Str(r) != "3.0" || r.TypeName() != "float" {
		t.Fatalf("super().__truediv__ = %v (%s), handled=%v, %v; want plain float 3.0", r, r.TypeName(), handled, err)
	}
	rr, handled, err := builtinBaseCall(f, "__rtruediv__", []Object{NewFloat(3.0)}, nil, nil)
	if !handled || err != nil || Str(rr) != "0.5" {
		t.Fatalf("super().__rtruediv__ = %v, handled=%v, %v; want 0.5", rr, handled, err)
	}

	cc := buildComplexSubclass(t, "MyComplex")
	z, err := Instantiate(cc, []Object{NewFloat(1), NewFloat(2)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyComplex(1, 2): %v", err)
	}
	cz, handled, err := builtinBaseCall(z, "__mul__", []Object{NewInt(2)}, nil, nil)
	if !handled || err != nil || cz.TypeName() != "complex" {
		t.Fatalf("super().__mul__ = %v (%s), handled=%v, %v; want a plain complex", cz, cz.TypeName(), handled, err)
	}

	// __init__ is not a base method the numeric payload answers, so builtinBaseCall
	// declines and leaves it to objectDefaultCall, keeping super().__init__() a
	// no-op rather than hijacking it with a numeric slot.
	if _, handled, _ := builtinBaseCall(f, "__init__", nil, nil, nil); handled {
		t.Fatal("super().__init__ on a float subclass was hijacked by the numeric resolver")
	}
}

// A numeric base arithmetic slot reads a numeric-subclass operand as its stored
// scalar and returns a plain result, so float.__add__ takes a float or int
// subclass and int.__add__ takes an int subclass, while an out-of-domain operand
// still declines with NotImplemented.
func TestNumericSlotAcceptsSubclassOperand(t *testing.T) {
	ff := buildFloatSubclass(t, "MyFloat")
	two, err := Instantiate(ff, []Object{NewFloat(2.0)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyFloat(2.0): %v", err)
	}
	// float.__add__(6.0, MyFloat(2.0)) reads the subclass as 2.0 and returns 8.0.
	r, err := CallMethod(NewFloat(6.0), "__add__", []Object{two})
	if err != nil || Str(r) != "8.0" || r.TypeName() != "float" {
		t.Fatalf("float.__add__(6.0, MyFloat(2.0)) = %v (%s), %v; want plain float 8.0", r, r.TypeName(), err)
	}

	// A complex operand is out of float's domain, so the slot declines.
	ni, err := CallMethod(NewFloat(6.0), "__add__", []Object{NewComplex(0, 1)})
	if err != nil || ni != NotImplemented {
		t.Fatalf("float.__add__(6.0, 1j) = %v, %v; want NotImplemented", ni, err)
	}

	fi := buildFloatSubclass(t, "AlsoFloat")
	// A float subclass is out of int's domain: int.__add__(6, MyFloat(2.0)) declines.
	twoF, err := Instantiate(fi, []Object{NewFloat(2.0)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate AlsoFloat(2.0): %v", err)
	}
	nf, err := CallMethod(NewInt(6), "__add__", []Object{twoF})
	if err != nil || nf != NotImplemented {
		t.Fatalf("int.__add__(6, MyFloat(2.0)) = %v, %v; want NotImplemented", nf, err)
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
