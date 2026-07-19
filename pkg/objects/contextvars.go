package objects

import (
	"fmt"
	"maps"
)

// contextvars models CPython's contextvars module (spec 2076 doc 10). A
// ContextVar is a key; its value lives in whichever context is current on the
// running thread, so the same variable reads differently under different
// contexts. The current context hangs off the Thread the call spine already
// carries, mirroring threading.local, since unagi passes the thread explicitly
// rather than consulting a goroutine-local slot.

// contextVar backs contextvars.ContextVar. It holds only its name and optional
// default; the value is stored per-context, not on the variable.
type contextVar struct {
	name       string
	hasDefault bool
	def        Object
}

// NewContextVar builds a ContextVar with the given name and, when hasDefault is
// set, a default get returns before any value is set.
func NewContextVar(name string, hasDefault bool, def Object) Object {
	return &contextVar{name: name, hasDefault: hasDefault, def: def}
}

func (*contextVar) TypeName() string { return "ContextVar" }

func (v *contextVar) repr() string {
	if v.hasDefault {
		return fmt.Sprintf("<ContextVar name %s default %s at %p>", quoteStr(v.name), Repr(v.def), v)
	}
	return fmt.Sprintf("<ContextVar name %s at %p>", quoteStr(v.name), v)
}

// contextObject backs contextvars.Context and doubles as a thread's current
// context. entries maps each set variable to its value; order tracks insertion
// so iteration is deterministic. entered guards a context against being run
// while it is already active.
type contextObject struct {
	entries map[*contextVar]Object
	order   []*contextVar
	entered bool
}

func newContext() *contextObject {
	return &contextObject{entries: make(map[*contextVar]Object)}
}

// NewEmptyContext builds a fresh contextvars.Context with no variables set, the
// object contextvars.Context() hands back.
func NewEmptyContext() Object { return newContext() }

func (*contextObject) TypeName() string { return "Context" }

// copy returns a shallow copy that shares no map with the original, what
// copy_context returns so later mutations under one context leave the other
// untouched.
func (c *contextObject) copy() *contextObject {
	n := &contextObject{
		entries: make(map[*contextVar]Object, len(c.entries)),
		order:   append([]*contextVar(nil), c.order...),
	}
	maps.Copy(n.entries, c.entries)
	return n
}

func (c *contextObject) lookup(v *contextVar) (Object, bool) {
	val, ok := c.entries[v]
	return val, ok
}

func (c *contextObject) assign(v *contextVar, val Object) {
	if _, ok := c.entries[v]; !ok {
		c.order = append(c.order, v)
	}
	c.entries[v] = val
}

func (c *contextObject) remove(v *contextVar) {
	if _, ok := c.entries[v]; !ok {
		return
	}
	delete(c.entries, v)
	for i, k := range c.order {
		if k == v {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (c *contextObject) repr() string { return fmt.Sprintf("<Context object at %p>", c) }

// keySlice, valSlice, and itemSlice materialise the set variables, their
// values, and (key, value) pairs in insertion order, backing len, iteration,
// and the keys/values/items views.
func (c *contextObject) keySlice() []Object {
	out := make([]Object, len(c.order))
	for i, v := range c.order {
		out[i] = v
	}
	return out
}

func (c *contextObject) valSlice() []Object {
	out := make([]Object, len(c.order))
	for i, v := range c.order {
		out[i] = c.entries[v]
	}
	return out
}

func (c *contextObject) itemSlice() []Object {
	out := make([]Object, len(c.order))
	for i, v := range c.order {
		out[i] = NewTuple([]Object{v, c.entries[v]})
	}
	return out
}

// getItem implements ctx[var]: the value var holds in this context. A missing
// variable is a KeyError and a non-ContextVar key is a TypeError, matching the
// mapping protocol CPython gives Context.
func (c *contextObject) getItem(key Object) (Object, error) {
	v, ok := key.(*contextVar)
	if !ok {
		return nil, Raise(TypeError, "a ContextVar key was expected, got %s", Repr(key))
	}
	if val, ok := c.lookup(v); ok {
		return val, nil
	}
	return nil, Raise(KeyError, "%s", v.repr())
}

// containsKey implements var in ctx. Like subscript, a non-ContextVar key is a
// TypeError rather than a plain false.
func (c *contextObject) containsKey(key Object) (Object, error) {
	v, ok := key.(*contextVar)
	if !ok {
		return nil, Raise(TypeError, "a ContextVar key was expected, got %s", Repr(key))
	}
	_, found := c.lookup(v)
	return NewBool(found), nil
}

// contextKeysObject, contextValuesObject, and contextItemsObject back
// Context.keys/values/items. Each holds the context and snapshots its variables
// lazily, so len and iteration read whatever the context holds when asked.
type contextKeysObject struct{ c *contextObject }

func (*contextKeysObject) TypeName() string { return "keys" }

func (k *contextKeysObject) repr() string { return fmt.Sprintf("<keys object at %p>", k) }

type contextValuesObject struct{ c *contextObject }

func (*contextValuesObject) TypeName() string { return "values" }

func (v *contextValuesObject) repr() string { return fmt.Sprintf("<values object at %p>", v) }

type contextItemsObject struct{ c *contextObject }

func (*contextItemsObject) TypeName() string { return "items" }

func (it *contextItemsObject) repr() string { return fmt.Sprintf("<items object at %p>", it) }

// CopyThreadContext returns a copy of thread t's current context, the value
// copy_context() gives back.
func CopyThreadContext(t *Thread) Object { return t.context().copy() }

// missingSentinel is the type of Token.MISSING, the singleton old_value a token
// carries when its variable held no value before the set that produced it.
type missingSentinel struct{}

func (*missingSentinel) TypeName() string { return "object" }

func (*missingSentinel) repr() string { return "<Token.MISSING>" }

var tokenMissing Object = &missingSentinel{}

// contextToken backs contextvars.Token, the receipt set returns and reset
// consumes. It pins the variable, the context it was made in, and the value to
// restore, so reset can refuse a token used twice or in the wrong context.
type contextToken struct {
	variable *contextVar
	ctx      *contextObject
	hadOld   bool
	oldValue Object
	used     bool
}

func (*contextToken) TypeName() string { return "Token" }

func (tk *contextToken) repr() string {
	used := ""
	if tk.used {
		used = " used"
	}
	return fmt.Sprintf("<Token%s var=%s at %p>", used, tk.variable.repr(), tk)
}

// tokenClass exposes contextvars.Token as a value so Token.MISSING resolves.
type tokenClass struct{}

func (*tokenClass) TypeName() string { return "type" }

var contextTokenClassValue Object = &tokenClass{}

// ContextTokenClass returns the contextvars.Token type object, exposed for the
// module so Token.MISSING can be read.
func ContextTokenClass() Object { return contextTokenClassValue }

// contextVarMethod dispatches ContextVar.get/set/reset against thread t's
// current context.
func contextVarMethod(t *Thread, v *contextVar, name string, args []Object) (Object, error) {
	ctx := t.context()
	switch name {
	case "get":
		if len(args) > 1 {
			return nil, Raise(TypeError, "get expected at most 1 argument, got %d", len(args))
		}
		if val, ok := ctx.lookup(v); ok {
			return val, nil
		}
		if len(args) == 1 {
			return args[0], nil
		}
		if v.hasDefault {
			return v.def, nil
		}
		return nil, Raise("LookupError", "%s", v.repr())
	case "set":
		if len(args) != 1 {
			return nil, Raise(TypeError, "set() takes exactly 1 argument (%d given)", len(args))
		}
		tok := &contextToken{variable: v, ctx: ctx}
		if old, ok := ctx.lookup(v); ok {
			tok.hadOld = true
			tok.oldValue = old
		}
		ctx.assign(v, args[0])
		return tok, nil
	case "reset":
		if len(args) != 1 {
			return nil, Raise(TypeError, "reset() takes exactly 1 argument (%d given)", len(args))
		}
		tok, ok := args[0].(*contextToken)
		if !ok {
			return nil, Raise(TypeError, "expected an instance of Token, got %s", args[0].TypeName())
		}
		if tok.used {
			return nil, Raise(RuntimeError, "%s has already been used once", tok.repr())
		}
		if tok.variable != v {
			return nil, Raise(ValueError, "%s was created by a different ContextVar", tok.repr())
		}
		if tok.ctx != ctx {
			return nil, Raise(ValueError, "%s was created in a different Context", tok.repr())
		}
		if tok.hadOld {
			ctx.assign(v, tok.oldValue)
		} else {
			ctx.remove(v)
		}
		tok.used = true
		return None, nil
	}
	return nil, Raise(AttributeError, "'ContextVar' object has no attribute '%s'", name)
}

// contextMethod dispatches Context.run and Context.get with no keywords.
func contextMethod(t *Thread, c *contextObject, name string, args []Object) (Object, error) {
	switch name {
	case "run":
		return c.run(t, args, nil, nil)
	case "get":
		return c.getMethod(args)
	case "keys":
		if len(args) != 0 {
			return nil, Raise(TypeError, "keys() takes no arguments (%d given)", len(args))
		}
		return &contextKeysObject{c: c}, nil
	case "values":
		if len(args) != 0 {
			return nil, Raise(TypeError, "values() takes no arguments (%d given)", len(args))
		}
		return &contextValuesObject{c: c}, nil
	case "items":
		if len(args) != 0 {
			return nil, Raise(TypeError, "items() takes no arguments (%d given)", len(args))
		}
		return &contextItemsObject{c: c}, nil
	}
	return nil, Raise(AttributeError, "'Context' object has no attribute '%s'", name)
}

// contextMethodKw dispatches Context.run with keyword arguments, which it
// forwards to the callable.
func contextMethodKw(t *Thread, c *contextObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name == "run" {
		return c.run(t, pos, kwNames, kwVals)
	}
	if len(kwNames) == 0 {
		return contextMethod(t, c, name, pos)
	}
	return nil, Raise(TypeError, "%s() takes no keyword arguments", name)
}

// getMethod implements Context.get(var[, default]): the value var holds in this
// context, or the default (None when omitted) when it is unset.
func (c *contextObject) getMethod(args []Object) (Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, Raise(TypeError, "get expected 1 to 2 arguments, got %d", len(args))
	}
	v, ok := args[0].(*contextVar)
	if !ok {
		return nil, Raise(TypeError, "a ContextVar key was expected, got %s", Repr(args[0]))
	}
	if val, ok := c.lookup(v); ok {
		return val, nil
	}
	if len(args) == 2 {
		return args[1], nil
	}
	return None, nil
}

// run enters this context on thread t, runs callable(*args, **kwargs) under it,
// and restores the previous context, whatever the call does. Variable changes
// made during the run stay in this context, so a later run of the same context
// sees them. A context already entered cannot be entered again.
func (c *contextObject) run(t *Thread, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) < 1 {
		return nil, Raise(TypeError, "run() missing 1 required positional argument: 'callable'")
	}
	if c.entered {
		return nil, Raise(RuntimeError, "cannot enter context: %s is already entered", c.repr())
	}
	fn := pos[0]
	prev := t.ctx
	t.ctx = c
	c.entered = true
	res, err := CallKwT(t, fn, pos[1:], kwNames, kwVals)
	c.entered = false
	t.ctx = prev
	return res, err
}

// quoteStr renders s the way Python reprs a string, the form ContextVar repr
// uses for its name.
func quoteStr(s string) string { return Repr(NewStr(s)) }
