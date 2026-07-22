package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestCollectionsAccelTypes proves the _collections container types are real
// type objects: they answer isinstance and issubclass, report as an instance of
// type, and are the value type() resolves an instance to. The vendored
// collections package leans on all of this when it registers deque with an abc.
func TestCollectionsAccelTypes(t *testing.T) {
	m, err := ImportModule("_collections")
	if err != nil {
		t.Fatalf("import _collections: %v", err)
	}
	for _, name := range []string{"deque", "defaultdict", "OrderedDict"} {
		cls, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("_collections.%s: %v", name, err)
		}
		isType, err := IsInstance(cls, BuiltinFn("type"))
		if err != nil {
			t.Fatalf("isinstance(%s, type): %v", name, err)
		}
		if isType != objects.True {
			t.Fatalf("_collections.%s is not a type", name)
		}
	}

	deque, _ := objects.LoadAttr(m, "deque")
	d, err := objects.Call(deque, []objects.Object{objects.NewList([]objects.Object{objects.NewInt(1)})})
	if err != nil {
		t.Fatalf("deque(...): %v", err)
	}
	inst, err := IsInstance(d, deque)
	if err != nil || inst != objects.True {
		t.Fatalf("isinstance(d, deque) = %v, %v", inst, err)
	}
	if TypeOf(d) != deque {
		t.Fatalf("type(d) is not deque")
	}
}

// TestWeakrefProxyForwards proves _weakref.proxy hands back a stand-in that
// reads through to the referent in this no-weak-GC tier, and rejects a value
// that cannot be referenced.
func TestWeakrefProxyForwards(t *testing.T) {
	m, err := ImportModule("_weakref")
	if err != nil {
		t.Fatalf("import _weakref: %v", err)
	}
	proxy, err := objects.LoadAttr(m, "proxy")
	if err != nil {
		t.Fatalf("_weakref.proxy: %v", err)
	}
	// A list carries no weak support, so proxy rejects it like ref does.
	if _, err := objects.Call(proxy, []objects.Object{objects.NewList(nil)}); err == nil {
		t.Fatalf("proxy(list) should raise TypeError")
	}
}
