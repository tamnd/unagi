package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// threading is a built-in module: CPython implements its core in C behind the
// _thread extension, so the runtime provides the identity surface in Go under
// the same import name. This first slice lands get_ident, the thread-identity
// primitive the rest of the module hangs off. The Thread object, the live
// registry (current_thread, main_thread, active_count, enumerate), and
// Thread.start arrive with the goroutine-spawn slice.

func init() {
	moduleTable["threading"] = &moduleEntry{builtin: true, exec: initThreading}
}

func initThreading(m *objects.Module) error {
	return objects.StoreAttr(m, "get_ident", objects.NewFunc("get_ident", -1, threadingGetIdent))
}

// threadingGetIdent is threading.get_ident(): the current thread's ident, a
// monotonically assigned int64 that is never reused within a process. The
// value is stricter than CPython, which may recycle idents, and therefore
// compatible with any program that only compares idents for equality or tests
// their type (spec 2076 doc 10 §2.1).
//
// The current thread is the one whose *objects.Thread the call spine carries.
// That thread is not yet delivered to a built-in: dynamic dispatch still routes
// the main thread and Thread.start does not exist, so the only live thread is
// the main thread and its ident is the current ident. When the spawn slice
// threads t into thread-sensitive built-ins, this reads t.Ident() instead; the
// value it returns for a single-threaded program is identical either way.
func threadingGetIdent(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "get_ident() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(objects.MainThread().Ident()), nil
}
