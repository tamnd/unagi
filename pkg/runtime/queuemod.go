package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// queue is a built-in module: CPython implements Queue in queue.py over the
// _thread primitives and the Empty exception in the _queue C extension, so the
// runtime provides the synchronized-FIFO surface in Go under the same import
// name. Queue builds the native queueObject; Empty and Full are the exceptions
// its non-blocking and timed calls raise, exposed here so a program can catch
// them by name.

func init() {
	moduleTable["queue"] = &moduleEntry{builtin: true, exec: initQueue}
}

func initQueue(m *objects.Module) error {
	for _, e := range []struct {
		name string
		fn   objects.Object
	}{
		{"Queue", objects.NewFuncKw("Queue", queueNewQueue)},
		{"Empty", objects.QueueEmptyClass()},
		{"Full", objects.QueueFullClass()},
	} {
		if err := objects.StoreAttr(m, e.name, e.fn); err != nil {
			return err
		}
	}
	return nil
}

// queueNewQueue is queue.Queue(maxsize=0): a synchronized FIFO. A maxsize of zero
// or below is CPython's unbounded queue, so a negative value is accepted the same
// way and reported back as zero.
func queueNewQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) > 1 {
		return nil, objects.Raise(objects.TypeError, "Queue() takes at most 1 argument (%d given)", len(pos))
	}
	var arg objects.Object
	if len(pos) == 1 {
		arg = pos[0]
	}
	for i, k := range kwNames {
		if k != "maxsize" {
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for Queue()", k)
		}
		if arg != nil {
			return nil, objects.Raise(objects.TypeError, "argument for Queue() given by name ('maxsize') and position")
		}
		arg = kwVals[i]
	}
	maxsize := int64(0)
	if arg != nil {
		v, ok := objects.AsInt(arg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", arg.TypeName())
		}
		maxsize = v
	}
	return objects.NewQueue(int(maxsize)), nil
}
