package objects

import "fmt"

// excTypeObject is the class object a with statement passes to __exit__ as its
// first argument on the exception path. unagi has no first-class builtin
// exception classes yet, so this is a minimal stand-in: it reprs like the real
// class and is a distinct, non-None object, which is what __exit__ bodies test
// when they branch on whether an exception occurred.
type excTypeObject struct{ name string }

func (*excTypeObject) TypeName() string { return "type" }

func excTypeRepr(t *excTypeObject) string { return fmt.Sprintf("<class '%s'>", t.name) }

// excTypeCache keeps one object per exception kind so repeated exits hand back
// the same class object. The emitted program is single-threaded until M5.
var excTypeCache = map[string]*excTypeObject{}

// ExcType returns the stand-in class object for a builtin exception kind.
func ExcType(kind string) Object {
	if t, ok := excTypeCache[kind]; ok {
		return t
	}
	t := &excTypeObject{name: kind}
	excTypeCache[kind] = t
	return t
}

// WithEnter runs the entry half of the context-manager protocol: it looks up
// __exit__ then __enter__ on the manager's type, both before either is called,
// and returns the bound __exit__ to run on the way out together with the
// result of __enter__. A type missing either method raises the protocol
// TypeError probed on 3.14, which names __exit__ first when both are absent.
func WithEnter(mgr Object) (exitFn Object, entered Object, err error) {
	inst, ok := mgr.(*instanceObject)
	if !ok {
		return nil, nil, Raise(TypeError,
			"'%s' object does not support the context manager protocol (missed __exit__ method)",
			mgr.TypeName())
	}
	exitM, ok := classMethod(inst.cls, "__exit__")
	if !ok {
		return nil, nil, Raise(TypeError,
			"'%s' object does not support the context manager protocol (missed __exit__ method)",
			inst.cls.name)
	}
	enterM, ok := classMethod(inst.cls, "__enter__")
	if !ok {
		return nil, nil, Raise(TypeError,
			"'%s' object does not support the context manager protocol (missed __enter__ method)",
			inst.cls.name)
	}
	entered, err = enterM.bind([]Object{mgr}, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	return &boundMethod{fn: exitM, self: mgr}, entered, nil
}

// classMethod resolves a dunder to a plain function on the class, the special
// method lookup the protocol uses: it ignores instance-dict entries and only a
// function on the class qualifies as a bound method.
func classMethod(c *classObject, name string) (*functionObject, bool) {
	v, ok := c.lookup(name)
	if !ok {
		return nil, false
	}
	fn, ok := v.(*functionObject)
	return fn, ok
}
