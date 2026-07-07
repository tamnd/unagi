package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// collections is a built-in module. CPython ships the accelerated container
// types in the C _collections module behind the pure-Python collections
// package, so the runtime owns the C types directly under the collections
// import name. deque is the first: a double-ended queue provided by
// objects.NewDeque.

func init() {
	moduleTable["collections"] = &moduleEntry{builtin: true, exec: initCollections}
}

func initCollections(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	// deque(iterable=(), maxlen=None): the iterable seeds the queue front to
	// back, and a non-negative maxlen bounds it so a later push at one end drops
	// an element from the other. Both arguments accept keywords, matching the C
	// signature.
	deque := objects.NewFunction("deque",
		[]objects.Param{
			{Name: "iterable", Kind: objects.ParamPlain},
			{Name: "maxlen", Kind: objects.ParamPlain},
		},
		[]objects.Object{objects.None, objects.None},
		func(a []objects.Object) (objects.Object, error) {
			maxlen, err := dequeMaxlen(a[1])
			if err != nil {
				return nil, err
			}
			var elts []objects.Object
			if a[0] != objects.None {
				elts, err = materialize(a[0])
				if err != nil {
					return nil, err
				}
			}
			return objects.NewDeque(elts, maxlen), nil
		})
	if err := set("deque", deque); err != nil {
		return err
	}

	return nil
}

// dequeMaxlen validates the maxlen argument: None is the unbounded sentinel
// (returned as -1), an integer must be non-negative, and any other type is the
// "an integer is required" TypeError CPython raises.
func dequeMaxlen(o objects.Object) (int, error) {
	if o == objects.None {
		return -1, nil
	}
	n, ok := objects.AsInt(o)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required")
	}
	if n < 0 {
		return 0, objects.Raise(objects.ValueError, "maxlen must be non-negative")
	}
	return int(n), nil
}
