package objects

import (
	"fmt"
	"sync"
)

// The pickle module's exception hierarchy: PickleError is the common base, and
// PicklingError (a dump failure) and UnpicklingError (a load failure) derive
// from it. CPython defines all three in the pickle module over the Exception
// base, so a program catches them by name; each is built once on first use,
// after excclass.go's init has populated the Exception base.

var (
	pickleErrorOnce      sync.Once
	pickleErrorClass     *classObject
	picklingErrorOnce    sync.Once
	picklingErrorClass   *classObject
	unpicklingErrorOnce  sync.Once
	unpicklingErrorClass *classObject
)

// PickleErrorClass returns pickle.PickleError, the base of the pickle
// exceptions.
func PickleErrorClass() Object {
	pickleErrorOnce.Do(func() {
		base, ok := ExcClass("Exception")
		if !ok {
			panic("unagi: Exception class unavailable for pickle.PickleError")
		}
		pickleErrorClass = buildPickleExc("PickleError", base)
	})
	return pickleErrorClass
}

// PicklingErrorClass returns pickle.PicklingError, raised when an object cannot
// be pickled.
func PicklingErrorClass() Object {
	picklingErrorOnce.Do(func() {
		picklingErrorClass = buildPickleExc("PicklingError", PickleErrorClass())
	})
	return picklingErrorClass
}

// UnpicklingErrorClass returns pickle.UnpicklingError, raised when a pickle
// stream is malformed or cannot be reconstructed.
func UnpicklingErrorClass() Object {
	unpicklingErrorOnce.Do(func() {
		unpicklingErrorClass = buildPickleExc("UnpicklingError", PickleErrorClass())
	})
	return unpicklingErrorClass
}

// buildPickleExc constructs one of the pickle exception classes against the
// given base, recording pickle as its module so __module__ and __qualname__
// read the way CPython reports them.
func buildPickleExc(name string, base Object) *classObject {
	c, err := NewClass(name, name, []Object{base}, []string{"__module__"}, []Object{NewStr("pickle")}, nil, nil)
	if err != nil {
		panic("unagi: building pickle." + name + ": " + err.Error())
	}
	return c.(*classObject)
}

// newUnpicklingError builds an UnpicklingError carrying a formatted message,
// ready to return as an error from a load.
func newUnpicklingError(format string, a ...any) error {
	return instantiatePickleExc(UnpicklingErrorClass(), fmt.Sprintf(format, a...))
}

// instantiatePickleExc builds an instance of a pickle exception class with a
// single message argument, so the class links through the instance for catching
// by name and the traceback renders "Name: msg".
func instantiatePickleExc(class Object, msg string) error {
	inst, err := Instantiate(class.(*classObject), []Object{NewStr(msg)}, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return Raise(class.(*classObject).name, "%s", msg)
}
