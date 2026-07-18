package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// queue is a built-in module: CPython implements Queue in queue.py over the
// _thread primitives and the Empty exception in the _queue C extension, so the
// runtime provides the synchronized-FIFO surface in Go under the same import
// name. Queue builds the native FIFO; LifoQueue and PriorityQueue are CPython's
// subclasses that swap the container for a stack and a heap. Empty and Full are the
// exceptions the non-blocking and timed calls raise, exposed here so a program can
// catch them by name.

func init() {
	moduleTable["queue"] = &moduleEntry{builtin: true, exec: initQueue}
}

func initQueue(m *objects.Module) error {
	for _, e := range []struct {
		name string
		fn   objects.Object
	}{
		{"Queue", objects.NewFuncKw("Queue", queueNewQueue)},
		{"LifoQueue", objects.NewFuncKw("LifoQueue", queueNewLifoQueue)},
		{"PriorityQueue", objects.NewFuncKw("PriorityQueue", queueNewPriorityQueue)},
		{"SimpleQueue", objects.NewFuncKw("SimpleQueue", queueNewSimpleQueue)},
		{"Empty", objects.QueueEmptyClass()},
		{"Full", objects.QueueFullClass()},
	} {
		if err := objects.StoreAttr(m, e.name, e.fn); err != nil {
			return err
		}
	}
	return nil
}

// queueNewQueue is queue.Queue(maxsize=0): a synchronized FIFO.
func queueNewQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	maxsize, err := queueParseMaxsize("Queue", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.NewQueue(maxsize), nil
}

// queueNewLifoQueue is queue.LifoQueue(maxsize=0): a synchronized stack.
func queueNewLifoQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	maxsize, err := queueParseMaxsize("LifoQueue", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.NewLifoQueue(maxsize), nil
}

// queueNewPriorityQueue is queue.PriorityQueue(maxsize=0): a synchronized heap.
func queueNewPriorityQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	maxsize, err := queueParseMaxsize("PriorityQueue", pos, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	return objects.NewPriorityQueue(maxsize), nil
}

// queueNewSimpleQueue is queue.SimpleQueue(): an unbounded FIFO with no task
// tracking. Unlike Queue it takes no maxsize, so it rejects any argument.
func queueNewSimpleQueue(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 0 || len(kwNames) != 0 {
		return nil, objects.Raise(objects.TypeError, "SimpleQueue() takes no arguments (%d given)", len(pos)+len(kwNames))
	}
	return objects.NewSimpleQueue(), nil
}

// queueParseMaxsize reads the shared (maxsize=0) constructor signature the three
// queue classes take. A maxsize of zero or below is CPython's unbounded queue, so a
// negative value is accepted the same way and reported back as zero.
func queueParseMaxsize(name string, pos []objects.Object, kwNames []string, kwVals []objects.Object) (int, error) {
	if len(pos) > 1 {
		return 0, objects.Raise(objects.TypeError, "%s() takes at most 1 argument (%d given)", name, len(pos))
	}
	var arg objects.Object
	if len(pos) == 1 {
		arg = pos[0]
	}
	for i, k := range kwNames {
		if k != "maxsize" {
			return 0, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for %s()", k, name)
		}
		if arg != nil {
			return 0, objects.Raise(objects.TypeError, "argument for %s() given by name ('maxsize') and position", name)
		}
		arg = kwVals[i]
	}
	if arg == nil {
		return 0, nil
	}
	v, ok := objects.AsInt(arg)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", arg.TypeName())
	}
	return int(v), nil
}
