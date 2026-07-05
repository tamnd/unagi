package objects

// A generator is a boxed, cooperatively scheduled coroutine. The emitted body
// runs on its own goroutine and hands control back and forth with the driver
// through two unbuffered channels, so exactly one side runs at a time and the
// object's fields need no locking. This gives full PEP 342 semantics, yield
// anywhere, send, throw, close, and yield from, at the cost of a goroutine per
// live generator. The static tier (M4+) replaces the goroutine with a resume
// switch state machine per D16; running one goroutine each is the honest boxed
// stand-in until then.
//
// The Done criterion "no goroutine is created per generator" is deliberately
// not met here: it is the static-tier target. This boxed generator is recorded
// as an accepted divergence against that milestone.

// Yielder is the handle a generator body uses to suspend. The emitted closure
// takes one and calls Yield for `yield e` and YieldFrom for `yield from e`.
type Yielder interface {
	Yield(v Object) (Object, error)
	YieldFrom(src Object) (Object, error)
}

// genSignal travels from the driver into the suspended body: val is the value a
// send resumes with, err is an exception a throw or close injects at the yield.
type genSignal struct {
	val Object
	err error
}

// genEvent travels from the body back to the driver: a yielded value, a
// propagating error, or completion carrying the return value.
type genEvent struct {
	val  Object
	err  error
	done bool
}

type generatorObject struct {
	qual    string
	body    func(Yielder) (Object, error)
	started bool
	done    bool
	running bool
	ret     Object // StopIteration value once the body has returned
	resume  chan genSignal
	out     chan genEvent
}

func (*generatorObject) TypeName() string { return "generator" }

// NewGenerator wraps a lowered generator body as a generator object. qual is
// the function's __qualname__, used only for repr.
func NewGenerator(qual string, body func(Yielder) (Object, error)) Object {
	return &generatorObject{qual: qual, body: body, ret: None}
}

// start launches the body goroutine, which blocks until the first resume so the
// driver and the body never run at once.
func (g *generatorObject) start() {
	g.resume = make(chan genSignal)
	g.out = make(chan genEvent)
	g.started = true
	go func() {
		<-g.resume // first resume just wakes the body; its value is discarded
		ret, err := g.body(g)
		if err != nil {
			g.out <- genEvent{err: pep479(err)}
			return
		}
		g.out <- genEvent{val: ret, done: true}
	}()
}

// Yield suspends the body: it hands v to the driver, then blocks until the
// driver resumes, returning the sent value or the injected exception.
func (g *generatorObject) Yield(v Object) (Object, error) {
	g.out <- genEvent{val: v}
	sig := <-g.resume
	if sig.err != nil {
		return nil, sig.err
	}
	return sig.val, nil
}

// YieldFrom delegates to a sub-iterable per PEP 380. A sub-generator forwards
// sent values and thrown exceptions and contributes its return value; any other
// iterable is driven with plain iteration and yields None as its value.
func (g *generatorObject) YieldFrom(src Object) (Object, error) {
	if sub, ok := src.(*generatorObject); ok {
		return g.delegate(sub)
	}
	it, err := Iter(src)
	if err != nil {
		return nil, err
	}
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			// PEP 380: the delegated-to iterator's StopIteration value is the
			// value of the yield-from expression. A built-in iterator carries
			// none and finishes as None; a user iterator surfaces the value its
			// __next__ raised through StopValue.
			if sv, ok := it.(stopValuer); ok {
				return sv.StopValue(), nil
			}
			return None, nil
		}
		if _, err := g.Yield(v); err != nil {
			return nil, err
		}
	}
}

// stopValuer is an iterator that carries a StopIteration value past exhaustion,
// so yield-from can bind it as its result. Generators and user __next__
// iterators implement it; built-in iterators do not and finish as None.
type stopValuer interface {
	StopValue() Object
}

// delegate runs a sub-generator to exhaustion, forwarding what the outer driver
// sends or throws and evaluating to the sub-generator's return value.
func (g *generatorObject) delegate(sub *generatorObject) (Object, error) {
	sig := genSignal{val: None}
	for {
		val, ret, done, err := sub.step(sig)
		if err != nil {
			return nil, err
		}
		if done {
			return ret, nil
		}
		sent, yerr := g.Yield(val)
		if yerr != nil {
			sig = genSignal{err: yerr}
			continue
		}
		sig = genSignal{val: sent}
	}
}

// step drives the generator one resume and reports what happened: a yielded
// value, or completion with the return value, or a propagating error.
func (g *generatorObject) step(sig genSignal) (val, ret Object, done bool, err error) {
	if g.done {
		// A throw into an exhausted generator still raises; a plain resume just
		// reports the finished state so send raises StopIteration.
		if sig.err != nil {
			return nil, nil, false, sig.err
		}
		return nil, g.ret, true, nil
	}
	if g.running {
		return nil, nil, false, Raise(ValueError, "generator already executing")
	}
	if !g.started {
		// A throw or close before the body ever ran skips it entirely.
		if sig.err != nil {
			g.done = true
			return nil, nil, false, sig.err
		}
		g.start()
	}
	g.running = true
	g.resume <- sig
	ev := <-g.out
	g.running = false
	if ev.done {
		g.done = true
		g.ret = ev.val
		return nil, ev.val, true, nil
	}
	if ev.err != nil {
		g.done = true
		return nil, nil, false, ev.err
	}
	return ev.val, nil, false, nil
}

// Iterate makes a generator its own iterator, so iter(g) is g.
func (g *generatorObject) Iterate() (Iterator, error) { return g, nil }

// Next advances the generator for the iterator protocol: a for loop or list()
// sees ordinary exhaustion when it finishes, and the carried return value stays
// reachable through StopValue for a next() that wants it.
func (g *generatorObject) Next() (Object, bool, error) {
	val, _, done, err := g.step(genSignal{val: None})
	if err != nil {
		return nil, false, err
	}
	if done {
		return nil, false, nil
	}
	return val, true, nil
}

// StopValue is the value the generator returned, so next() can raise
// StopIteration carrying it once the generator is exhausted.
func (g *generatorObject) StopValue() Object { return g.ret }

// send resumes the generator with a value. Sending anything but None before the
// first yield is the error CPython raises, since there is no yield to receive
// it. Completion raises StopIteration carrying the return value.
func (g *generatorObject) send(v Object) (Object, error) {
	if !g.started && v != None {
		return nil, Raise(TypeError, "can't send non-None value to a just-started generator")
	}
	val, ret, done, err := g.step(genSignal{val: v})
	if err != nil {
		return nil, err
	}
	if done {
		return nil, stopIteration(ret)
	}
	return val, nil
}

// throwInto injects an exception at the current yield. The body may catch it
// and yield again, let it propagate, or finish; a finish raises StopIteration.
func (g *generatorObject) throwInto(exc *Exception) (Object, error) {
	val, ret, done, err := g.step(genSignal{err: exc})
	if err != nil {
		return nil, err
	}
	if done {
		return nil, stopIteration(ret)
	}
	return val, nil
}

// closeGen injects GeneratorExit. A generator that exits cleanly, by letting the
// exit propagate or returning, closes to None; one that yields again is the
// RuntimeError CPython raises, and one that raises StopIteration is the PEP 479
// "generator raised StopIteration" RuntimeError, converted at the frame boundary
// before it reaches here.
func (g *generatorObject) closeGen() (Object, error) {
	if g.done || !g.started {
		g.done = true
		return None, nil
	}
	val, _, done, err := g.step(genSignal{err: &Exception{Kind: "GeneratorExit"}})
	if err != nil {
		if e, ok := err.(*Exception); ok && e.Kind == "GeneratorExit" {
			return None, nil
		}
		return nil, err
	}
	if done {
		return None, nil
	}
	_ = val
	return nil, Raise(RuntimeError, "generator ignored GeneratorExit")
}

// genMethod dispatches g.send, g.throw, and g.close.
func genMethod(g *generatorObject, name string, args []Object) (Object, error) {
	switch name {
	case "send":
		if len(args) != 1 {
			return nil, Raise(TypeError, "send() takes exactly one argument (%d given)", len(args))
		}
		return g.send(args[0])
	case "throw":
		exc, err := throwValue(args)
		if err != nil {
			return nil, err
		}
		return g.throwInto(exc)
	case "close":
		if len(args) != 0 {
			return nil, Raise(TypeError, "close() takes no arguments (%d given)", len(args))
		}
		return g.closeGen()
	}
	return nil, noAttr(g, name)
}

// NextValue implements the next() builtin: next(it) or next(it, default). The
// argument must already be an iterator, the type CPython insists on; a list or
// other iterable is not one until iter() wraps it. Exhaustion raises
// StopIteration, or returns the default when one is given, and a generator's
// return value rides the raised StopIteration.
func NextValue(args []Object) (Object, error) {
	switch len(args) {
	case 0:
		return nil, Raise(TypeError, "next expected at least 1 argument, got 0")
	case 1, 2:
	default:
		return nil, Raise(TypeError, "next expected at most 2 arguments, got %d", len(args))
	}
	if inst, iok := args[0].(*instanceObject); iok {
		if _, has := inst.cls.lookup("__next__"); has {
			res, _, err := instanceSpecial(inst, "__next__")
			if err != nil {
				if ex, exok := err.(*Exception); exok && ex.Kind == "StopIteration" && len(args) == 2 {
					return args[1], nil
				}
				return nil, err
			}
			return res, nil
		}
		return nil, Raise(TypeError, "'%s' object is not an iterator", args[0].TypeName())
	}
	it, ok := args[0].(Iterator)
	if !ok {
		return nil, Raise(TypeError, "'%s' object is not an iterator", args[0].TypeName())
	}
	v, ok, err := it.Next()
	if err != nil {
		return nil, err
	}
	if ok {
		return v, nil
	}
	if len(args) == 2 {
		return args[1], nil
	}
	if g, gok := args[0].(*generatorObject); gok {
		return nil, stopIteration(g.ret)
	}
	return nil, &Exception{Kind: "StopIteration"}
}

// stopIteration builds the StopIteration a completed generator raises. A None
// return carries no argument, matching a bare `return`; any other value becomes
// the single argument, so `except StopIteration as e: e.value` reads it back.
func stopIteration(v Object) *Exception {
	if v == nil || v == None {
		return &Exception{Kind: "StopIteration"}
	}
	return &Exception{Kind: "StopIteration", Args: []Object{v}}
}

// excStopValue reads the value a StopIteration carried, the inverse of
// stopIteration: the first argument when one is present, None otherwise. It is
// the result a yield-from binds when the delegated-to iterator finishes.
func excStopValue(e *Exception) Object {
	if len(e.Args) > 0 {
		return e.Args[0]
	}
	return None
}

// pep479 converts a StopIteration that escapes a generator frame into
// RuntimeError, PEP 479. A StopIteration raised inside a generator body must not
// leak out as ordinary exhaustion, so it becomes "generator raised
// StopIteration" carrying the original as both __cause__ and __context__ with
// context suppressed, matching CPython's gen_send_ex2. Any other error, a normal
// return, or a GeneratorExit passes through unchanged.
func pep479(err error) error {
	e, ok := err.(*Exception)
	if !ok || e.Kind != "StopIteration" {
		return err
	}
	re := Raise(RuntimeError, "generator raised StopIteration")
	re.Cause = e
	re.Context = e
	re.SuppressContext = true
	return re
}

// throwValue builds the exception a generator throw injects, applying the
// type/value normalization CPython's _PyErr_CreateException performs. The
// single-argument form takes an exception instance or a class to instantiate.
// The deprecated two- and three-argument forms take a class plus a value and an
// ignored traceback: the class is instantiated with the value unless the value
// is already an instance of it, a tuple splats into the constructor arguments,
// and None means no arguments. unagi models no traceback object, so the third
// argument is accepted and ignored.
func throwValue(args []Object) (*Exception, error) {
	switch {
	case len(args) == 0:
		return nil, Raise(TypeError, "throw expected at least 1 argument, got 0")
	case len(args) > 3:
		return nil, Raise(TypeError, "throw expected at most 3 arguments, got %d", len(args))
	}
	typ := args[0]
	value := None
	if len(args) >= 2 {
		value = args[1]
	}
	// An exception instance is thrown as itself and may not carry a separate
	// value, the way `raise inst` takes no second operand.
	if e, ok := typ.(*Exception); ok {
		if value != None {
			return nil, Raise(TypeError, "instance exception may not have a separate value")
		}
		return e, nil
	}
	c, ok := typ.(*classObject)
	if !ok || !isExcClass(c) {
		return nil, Raise(TypeError, "exceptions must be classes or instances deriving from BaseException, not %s", typ.TypeName())
	}
	// A value already an instance of the class is used unchanged; anything else
	// becomes the constructor arguments, a tuple splatting into them.
	if e, ok := value.(*Exception); ok && ExcMatchesClass(e, c) {
		return e, nil
	}
	var pos []Object
	switch {
	case value == None:
		pos = nil
	case isTupleValue(value):
		pos = value.(*tupleObject).elts
	default:
		pos = []Object{value}
	}
	inst, err := Instantiate(c, pos, nil, nil)
	if err != nil {
		return nil, err
	}
	e, ok := inst.(*Exception)
	if !ok {
		return nil, Raise(TypeError, "calling %s should have returned an instance of BaseException, not %s", c.TypeName(), inst.TypeName())
	}
	return e, nil
}

// isTupleValue reports whether o is a tuple, the sequence a throw splats into
// the exception constructor arguments.
func isTupleValue(o Object) bool {
	_, ok := o.(*tupleObject)
	return ok
}
