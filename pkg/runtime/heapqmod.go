package runtime

import "github.com/tamnd/unagi/pkg/objects"

// _heapq is the C accelerator behind the public heapq module. heapq.py defines
// heappush/heappop/heapify and their friends in pure Python, then does
// `from _heapq import *` (guarded by `except ImportError: pass`) to overwrite
// them with the faster C versions; test_heapq.TestHeapC pins that C variant
// specifically. The heap is a binary min-heap stored in a Python list with the
// invariant a[k] <= a[2*k+1] and a[k] <= a[2*k+2]; the _max variants keep the
// mirror-image maxheap. Every ordering decision goes through the same `<`
// comparison protocol list.sort and bisect use, so a custom __lt__ (the priority
// hiding in a (key, record) tuple, say) drives the heap the way it does the sort.
//
// The functions require an actual list, like CPython's PyList_Check: they index
// and assign in place and call the list's own append/pop, so a non-list raises
// TypeError before any work happens.

func init() {
	moduleTable["_heapq"] = &moduleEntry{builtin: true, exec: initHeapq}
}

func initHeapq(m *objects.Module) error {
	fns := []struct {
		name  string
		arity int
		fn    func(args []objects.Object) (objects.Object, error)
	}{
		{"heappush", 2, heappush},
		{"heappop", 1, heappop},
		{"heapreplace", 2, heapreplace},
		{"heappushpop", 2, heappushpop},
		{"heapify", 1, heapify},
		{"heappush_max", 2, heappushMax},
		{"heappop_max", 1, heappopMax},
		{"heapreplace_max", 2, heapreplaceMax},
		{"heappushpop_max", 2, heappushpopMax},
		{"heapify_max", 1, heapifyMax},
	}
	for _, f := range fns {
		if err := objects.StoreAttr(m, f.name, objects.NewFunc(f.name, f.arity, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// heapArg validates that arg 1 is a list, matching PyList_Check, and returns its
// current length so the callers can branch on emptiness without re-fetching it.
func heapArg(name string, heap objects.Object) (int, error) {
	if !objects.IsList(heap) {
		return 0, objects.Raise(objects.TypeError,
			"%s() argument 1 must be list, not %s", name, heap.TypeName())
	}
	return objects.Len(heap)
}

// hget and hset read and overwrite a heap slot through the ordinary sequence
// protocol; on a real list these are the in-place array accesses the C code does.
func hget(heap objects.Object, i int) (objects.Object, error) {
	return objects.GetItem(heap, objects.NewInt(int64(i)))
}

func hset(heap objects.Object, i int, v objects.Object) error {
	return objects.SetItem(heap, objects.NewInt(int64(i)), v)
}

func hpush(heap, item objects.Object) error {
	_, err := objects.CallMethod(heap, "append", []objects.Object{item})
	return err
}

// hpopLast removes and returns the final element, heap.pop() with no argument.
func hpopLast(heap objects.Object) (objects.Object, error) {
	return objects.CallMethod(heap, "pop", nil)
}

// less reports a < b through the full rich-comparison protocol, the shared
// ordering primitive defined alongside bisect's lessThan.
func less(a, b objects.Object) (bool, error) { return lessThan(a, b) }

// siftdown restores the min-heap invariant on the leaf at pos by walking toward
// startpos, moving parents down until newitem finds its place. maxHeap flips the
// comparison so the same code keeps a maxheap.
func siftdown(heap objects.Object, startpos, pos int, maxHeap bool) error {
	newitem, err := hget(heap, pos)
	if err != nil {
		return err
	}
	for pos > startpos {
		parentpos := (pos - 1) >> 1
		parent, err := hget(heap, parentpos)
		if err != nil {
			return err
		}
		// min: newitem < parent moves parent down; max: parent < newitem does.
		var lt bool
		if maxHeap {
			lt, err = less(parent, newitem)
		} else {
			lt, err = less(newitem, parent)
		}
		if err != nil {
			return err
		}
		if !lt {
			break
		}
		if err := hset(heap, pos, parent); err != nil {
			return err
		}
		pos = parentpos
	}
	return hset(heap, pos, newitem)
}

// siftup bubbles the smaller (min) or larger (max) child up from pos to a leaf,
// then sifts the element originally at pos down into the vacated slot -- the
// comparison-thrifty shape CPython documents at length in heapq.py.
func siftup(heap objects.Object, pos int, maxHeap bool) error {
	endpos, err := objects.Len(heap)
	if err != nil {
		return err
	}
	startpos := pos
	newitem, err := hget(heap, pos)
	if err != nil {
		return err
	}
	childpos := 2*pos + 1
	for childpos < endpos {
		rightpos := childpos + 1
		if rightpos < endpos {
			left, err := hget(heap, childpos)
			if err != nil {
				return err
			}
			right, err := hget(heap, rightpos)
			if err != nil {
				return err
			}
			// min: pick right unless left < right (i.e. not left<right -> right).
			// max: pick right unless right < left.
			var takeRight bool
			if maxHeap {
				lt, err := less(right, left)
				if err != nil {
					return err
				}
				takeRight = !lt
			} else {
				lt, err := less(left, right)
				if err != nil {
					return err
				}
				takeRight = !lt
			}
			if takeRight {
				childpos = rightpos
			}
		}
		child, err := hget(heap, childpos)
		if err != nil {
			return err
		}
		if err := hset(heap, pos, child); err != nil {
			return err
		}
		pos = childpos
		childpos = 2*pos + 1
	}
	if err := hset(heap, pos, newitem); err != nil {
		return err
	}
	return siftdown(heap, startpos, pos, maxHeap)
}

// pushImpl appends item and sifts it down toward the root.
func pushImpl(name string, args []objects.Object, maxHeap bool) (objects.Object, error) {
	heap := args[0]
	n, err := heapArg(name, heap)
	if err != nil {
		return nil, err
	}
	if err := hpush(heap, args[1]); err != nil {
		return nil, err
	}
	return objects.None, siftdown(heap, 0, n, maxHeap)
}

// popImpl removes the last element, and if the heap is now non-empty, swaps it
// into the root and sifts it up, returning the old root.
func popImpl(name string, args []objects.Object, maxHeap bool) (objects.Object, error) {
	heap := args[0]
	if _, err := heapArg(name, heap); err != nil {
		return nil, err
	}
	last, err := hpopLast(heap)
	if err != nil {
		return nil, err
	}
	n, err := objects.Len(heap)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return last, nil
	}
	returnitem, err := hget(heap, 0)
	if err != nil {
		return nil, err
	}
	if err := hset(heap, 0, last); err != nil {
		return nil, err
	}
	return returnitem, siftup(heap, 0, maxHeap)
}

// replaceImpl pops the root and pushes item in one sift, returning the old root;
// an empty heap has no root to return, so it raises IndexError.
func replaceImpl(name string, args []objects.Object, maxHeap bool) (objects.Object, error) {
	heap := args[0]
	n, err := heapArg(name, heap)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, objects.Raise(objects.IndexError, "index out of range")
	}
	returnitem, err := hget(heap, 0)
	if err != nil {
		return nil, err
	}
	if err := hset(heap, 0, args[1]); err != nil {
		return nil, err
	}
	return returnitem, siftup(heap, 0, maxHeap)
}

// pushpopImpl pushes item then pops the root in one shot; when item already
// belongs at or below the root it is returned unchanged without touching the heap.
func pushpopImpl(name string, args []objects.Object, maxHeap bool) (objects.Object, error) {
	heap, item := args[0], args[1]
	n, err := heapArg(name, heap)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return item, nil
	}
	root, err := hget(heap, 0)
	if err != nil {
		return nil, err
	}
	// min: swap only if root < item; max: swap only if item < root.
	var swap bool
	if maxHeap {
		swap, err = less(item, root)
	} else {
		swap, err = less(root, item)
	}
	if err != nil {
		return nil, err
	}
	if !swap {
		return item, nil
	}
	if err := hset(heap, 0, item); err != nil {
		return nil, err
	}
	return root, siftup(heap, 0, maxHeap)
}

// heapifyImpl sifts up every internal node from the bottom, turning an arbitrary
// list into a heap in O(n).
func heapifyImpl(name string, args []objects.Object, maxHeap bool) (objects.Object, error) {
	heap := args[0]
	n, err := heapArg(name, heap)
	if err != nil {
		return nil, err
	}
	for i := n/2 - 1; i >= 0; i-- {
		if err := siftup(heap, i, maxHeap); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

func heappush(args []objects.Object) (objects.Object, error) {
	return pushImpl("heappush", args, false)
}
func heappop(args []objects.Object) (objects.Object, error) {
	return popImpl("heappop", args, false)
}
func heapreplace(args []objects.Object) (objects.Object, error) {
	return replaceImpl("heapreplace", args, false)
}
func heappushpop(args []objects.Object) (objects.Object, error) {
	return pushpopImpl("heappushpop", args, false)
}
func heapify(args []objects.Object) (objects.Object, error) {
	return heapifyImpl("heapify", args, false)
}
func heappushMax(args []objects.Object) (objects.Object, error) {
	return pushImpl("heappush_max", args, true)
}
func heappopMax(args []objects.Object) (objects.Object, error) {
	return popImpl("heappop_max", args, true)
}
func heapreplaceMax(args []objects.Object) (objects.Object, error) {
	return replaceImpl("heapreplace_max", args, true)
}
func heappushpopMax(args []objects.Object) (objects.Object, error) {
	return pushpopImpl("heappushpop_max", args, true)
}
func heapifyMax(args []objects.Object) (objects.Object, error) {
	return heapifyImpl("heapify_max", args, true)
}
