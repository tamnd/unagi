package runtime

import "github.com/tamnd/unagi/pkg/objects"

// gc is the interface to CPython's cyclic garbage collector. unagi programs run
// on Go's runtime, whose collector owns memory and reclaims cycles on its own,
// so there is no separate generational collector to drive. This module exposes
// the gc surface the stdlib touches with the honest semantics that mapping
// allows: the enable/disable flag is a real toggle a program can read back,
// collect finds nothing unreachable to report, and the object-graph
// introspection reports empty because unagi keeps no CPython object graph to
// walk.
//
// timeit disables the collector around a timing loop
// (gc.isenabled/gc.disable/gc.enable) and trace calls gc.get_referrers to map a
// code object back to its function names; both imported nothing but a missing
// module before this. The control surface timeit drives is faithful; the
// introspection surface trace's coverage annotation leans on returns empty,
// which degrades that one annotation rather than raising.

// gcEnabled models CPython's collector-enabled flag. Go's collector runs
// regardless; this only records what a program set so isenabled reads it back.
var gcEnabled = true

func init() {
	moduleTable["gc"] = &moduleEntry{builtin: true, exec: initGC}
}

func initGC(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	fns := []struct {
		name  string
		arity int
		fn    func(args []objects.Object) (objects.Object, error)
	}{
		{"enable", 0, gcEnable},
		{"disable", 0, gcDisable},
		{"isenabled", 0, gcIsEnabled},
		{"collect", -1, gcCollect},
		{"get_count", 0, gcGetCount},
		{"get_threshold", 0, gcGetThreshold},
		{"set_threshold", -1, gcNone},
		{"get_debug", 0, gcZero},
		{"set_debug", -1, gcNone},
		{"get_objects", -1, gcEmptyList},
		{"get_referrers", -1, gcEmptyList},
		{"get_referents", -1, gcEmptyList},
		{"get_stats", 0, gcGetStats},
		{"is_tracked", 1, gcIsTracked},
		{"is_finalized", 1, gcFalse},
		{"freeze", 0, gcNone},
		{"unfreeze", 0, gcNone},
		{"get_freeze_count", 0, gcZero},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, f.arity, f.fn)); err != nil {
			return err
		}
	}
	// garbage holds uncollectable objects; unagi never reports any, so it is a
	// live empty list a program can inspect. callbacks is the collector's
	// callback list, empty for the same reason.
	if err := set("garbage", objects.NewList(nil)); err != nil {
		return err
	}
	if err := set("callbacks", objects.NewList(nil)); err != nil {
		return err
	}
	// The DEBUG_* flags CPython exposes for set_debug. They are accepted and
	// stored back through get_debug's zero, but kept as the constants a program
	// reads by name.
	consts := []struct {
		name string
		v    int64
	}{
		{"DEBUG_STATS", 1},
		{"DEBUG_COLLECTABLE", 2},
		{"DEBUG_UNCOLLECTABLE", 4},
		{"DEBUG_SAVEALL", 32},
		{"DEBUG_LEAK", 2 | 4 | 32},
	}
	for _, c := range consts {
		if err := set(c.name, objects.NewInt(c.v)); err != nil {
			return err
		}
	}
	return nil
}

func gcEnable([]objects.Object) (objects.Object, error) {
	gcEnabled = true
	return objects.None, nil
}

func gcDisable([]objects.Object) (objects.Object, error) {
	gcEnabled = false
	return objects.None, nil
}

func gcIsEnabled([]objects.Object) (objects.Object, error) {
	return objects.NewBool(gcEnabled), nil
}

// gcCollect reports zero unreachable objects found: Go's collector reclaims
// cycles itself, so a run of CPython's collector has nothing to do.
func gcCollect([]objects.Object) (objects.Object, error) {
	return objects.NewInt(0), nil
}

// gcGetCount reports the three generation counts as zero, the state right after
// a collection, since unagi drives no generational bookkeeping.
func gcGetCount([]objects.Object) (objects.Object, error) {
	return objects.NewTuple([]objects.Object{objects.NewInt(0), objects.NewInt(0), objects.NewInt(0)}), nil
}

// gcGetThreshold reports CPython's default collection thresholds so a program
// reading them sees the documented (700, 10, 10).
func gcGetThreshold([]objects.Object) (objects.Object, error) {
	return objects.NewTuple([]objects.Object{objects.NewInt(700), objects.NewInt(10), objects.NewInt(10)}), nil
}

// gcGetStats reports one empty stats record per generation, each with the keys
// CPython uses, all zero because no collection ran.
func gcGetStats([]objects.Object) (objects.Object, error) {
	stats := make([]objects.Object, 3)
	for i := range stats {
		d, err := objects.NewDict(
			[]objects.Object{objects.NewStr("collections"), objects.NewStr("collected"), objects.NewStr("uncollectable")},
			[]objects.Object{objects.NewInt(0), objects.NewInt(0), objects.NewInt(0)},
		)
		if err != nil {
			return nil, err
		}
		stats[i] = d
	}
	return objects.NewList(stats), nil
}

// gcIsTracked answers whether the collector would track an object: the builtin
// containers that can take part in a cycle report True, atomic values report
// False, the split CPython makes for these types. It classifies by type name, so
// a bare user instance reports False here where CPython reports True; the
// distinction is not on any import path unagi drives and no target module reads
// it, so the coarse answer stands.
func gcIsTracked(args []objects.Object) (objects.Object, error) {
	switch args[0].TypeName() {
	case "list", "dict", "set", "tuple", "OrderedDict", "defaultdict", "Counter":
		return objects.NewBool(true), nil
	}
	return objects.NewBool(false), nil
}

func gcEmptyList([]objects.Object) (objects.Object, error) {
	return objects.NewList(nil), nil
}

func gcNone([]objects.Object) (objects.Object, error) {
	return objects.None, nil
}

func gcZero([]objects.Object) (objects.Object, error) {
	return objects.NewInt(0), nil
}

func gcFalse([]objects.Object) (objects.Object, error) {
	return objects.NewBool(false), nil
}
