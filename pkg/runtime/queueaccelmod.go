package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _queue is the C accelerator behind the public queue module. queue.py does
// `from _queue import Empty, SimpleQueue` (guarded by `except ImportError`) and
// falls back to a pure-Python _PySimpleQueue only when the import fails;
// test_queue's C_SimpleQueueTest pins the accelerated SimpleQueue by importing
// it from _queue directly. Empty is likewise defined in the C module and
// re-exported by queue.py, so `queue.Empty is _queue.Empty`.
//
// unagi implements the queue surface natively in Go rather than vendoring
// queue.py, so both names already exist under the queue module. This module
// exposes the very same objects under the _queue name -- the shared SimpleQueue
// constructor and the QueueEmptyClass singleton -- so importing them from either
// place yields identical objects, matching CPython's identity. SimpleQueue's
// __module__ (via the object's type) and Empty's __module__ already read _queue.

func init() {
	moduleTable["_queue"] = &moduleEntry{builtin: true, exec: initQueueAccel}
}

func initQueueAccel(m *objects.Module) error {
	if err := objects.StoreAttr(m, "SimpleQueue", queueSimpleQueueCtor); err != nil {
		return err
	}
	return objects.StoreAttr(m, "Empty", objects.QueueEmptyClass())
}
