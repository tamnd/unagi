//go:build darwin || linux

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// resource is the POSIX process-resource accelerator. test.support suppresses
// core dumps with setrlimit(RLIMIT_CORE, (0, 0)), memory-hungry tests cap the
// address space with RLIMIT_AS, servers raise RLIMIT_NOFILE, and profiling reads
// CPU time and peak RSS out of getrusage(RUSAGE_SELF). It is a thin wrapper over
// getrusage(2), getrlimit(2)/setrlimit(2) and getpagesize(3) plus the RLIMIT_*
// and RUSAGE_* constants, taken from Go's syscall package so they carry the
// host's real values the way CPython takes them from <sys/resource.h>. The
// Rusage and Rlimit struct layouts have the same field names on darwin and
// linux, so the module is a single file scoped to those two hosts; the only
// platform seam is the handful of host-specific rlimit constants in
// resourcePlatformConsts.
//
// getrlimit returns and setrlimit accepts the (soft, hard) pair as CPython does,
// with an unlimited value expressed as RLIM_INFINITY. That constant is the
// host's own (int64 max on darwin, -1 on linux), and a plain int64<->uint64 cast
// carries an unlimited limit through unchanged on both, so no special-casing is
// needed: a program that reads RLIM_INFINITY back and passes it to setrlimit
// gets the same unlimited limit.

func init() {
	moduleTable["resource"] = &moduleEntry{builtin: true, exec: initResource}
}

// resourceStructRusage is the structseq getrusage returns: 16 fields, all named
// and in the sequence in CPython's order, so the value unpacks as a 16-tuple and
// answers ru_utime/ru_maxrss/... alike. The two time fields are floats (seconds),
// the rest are ints.
var resourceStructRusage = objects.NewStructSeqType(
	"struct_rusage", "resource.struct_rusage",
	[]string{
		"ru_utime", "ru_stime", "ru_maxrss", "ru_ixrss", "ru_idrss", "ru_isrss",
		"ru_minflt", "ru_majflt", "ru_nswap", "ru_inblock", "ru_oublock",
		"ru_msgsnd", "ru_msgrcv", "ru_nsignals", "ru_nvcsw", "ru_nivcsw",
	},
	16, 0,
)

// resourceConsts is the name->value table shared by darwin and linux. Every
// value comes from syscall so it is the host's real number; resourcePlatformConsts
// adds the host-specific extras (RSS/NPROC/MEMLOCK, which Go's syscall does not
// define on every host).
var resourceConsts = []struct {
	name string
	val  int
}{
	{"RLIMIT_CPU", syscall.RLIMIT_CPU},
	{"RLIMIT_FSIZE", syscall.RLIMIT_FSIZE},
	{"RLIMIT_DATA", syscall.RLIMIT_DATA},
	{"RLIMIT_STACK", syscall.RLIMIT_STACK},
	{"RLIMIT_CORE", syscall.RLIMIT_CORE},
	{"RLIMIT_NOFILE", syscall.RLIMIT_NOFILE},
	{"RLIMIT_AS", syscall.RLIMIT_AS},
	{"RUSAGE_SELF", syscall.RUSAGE_SELF},
	{"RUSAGE_CHILDREN", syscall.RUSAGE_CHILDREN},
}

func initResource(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("struct_rusage", resourceStructRusage); err != nil {
		return err
	}
	if err := set("getrusage", objects.NewFunc("getrusage", 1, resourceGetrusage)); err != nil {
		return err
	}
	if err := set("getrlimit", objects.NewFunc("getrlimit", 1, resourceGetrlimit)); err != nil {
		return err
	}
	if err := set("setrlimit", objects.NewFunc("setrlimit", 2, resourceSetrlimit)); err != nil {
		return err
	}
	if err := set("getpagesize", objects.NewFunc("getpagesize", 0, resourceGetpagesize)); err != nil {
		return err
	}
	// resource.error is an alias for OSError, exactly as CPython aliases it.
	if oserr, ok := objects.ExcClassValue("OSError"); ok {
		if err := set("error", oserr); err != nil {
			return err
		}
	}
	if err := set("RLIM_INFINITY", objects.NewInt(int64(syscall.RLIM_INFINITY))); err != nil {
		return err
	}
	for _, c := range resourceConsts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	for _, c := range resourcePlatformConsts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	return nil
}

// timevalSeconds turns a Timeval into float seconds, the ru_utime/ru_stime shape.
func timevalSeconds(tv syscall.Timeval) float64 {
	return float64(tv.Sec) + float64(tv.Usec)/1e6
}

// resourceGetrusage is resource.getrusage(who): the struct_rusage for the current
// process (RUSAGE_SELF) or its reaped children (RUSAGE_CHILDREN).
func resourceGetrusage(args []objects.Object) (objects.Object, error) {
	who, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getrusage() argument must be an int")
	}
	var ru syscall.Rusage
	if err := syscall.Getrusage(int(who), &ru); err != nil {
		return nil, objects.Raise(objects.ValueError, "invalid who parameter")
	}
	vals := []objects.Object{
		objects.NewFloat(timevalSeconds(ru.Utime)),
		objects.NewFloat(timevalSeconds(ru.Stime)),
		objects.NewInt(int64(ru.Maxrss)),
		objects.NewInt(int64(ru.Ixrss)),
		objects.NewInt(int64(ru.Idrss)),
		objects.NewInt(int64(ru.Isrss)),
		objects.NewInt(int64(ru.Minflt)),
		objects.NewInt(int64(ru.Majflt)),
		objects.NewInt(int64(ru.Nswap)),
		objects.NewInt(int64(ru.Inblock)),
		objects.NewInt(int64(ru.Oublock)),
		objects.NewInt(int64(ru.Msgsnd)),
		objects.NewInt(int64(ru.Msgrcv)),
		objects.NewInt(int64(ru.Nsignals)),
		objects.NewInt(int64(ru.Nvcsw)),
		objects.NewInt(int64(ru.Nivcsw)),
	}
	return resourceStructRusage.NewStructSeq(vals, vals), nil
}

// resourceGetrlimit is resource.getrlimit(resource): the (soft, hard) pair for a
// limit, with an unlimited value as RLIM_INFINITY.
func resourceGetrlimit(args []objects.Object) (objects.Object, error) {
	which, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getrlimit() argument must be an int")
	}
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(int(which), &lim); err != nil {
		return nil, objects.Raise(objects.ValueError, "invalid resource specified")
	}
	return objects.NewTuple([]objects.Object{
		objects.NewInt(int64(lim.Cur)),
		objects.NewInt(int64(lim.Max)),
	}), nil
}

// resourceSetrlimit is resource.setrlimit(resource, (soft, hard)): sets a limit
// from a two-item (soft, hard) sequence, RLIM_INFINITY meaning unlimited.
func resourceSetrlimit(args []objects.Object) (objects.Object, error) {
	which, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "setrlimit() argument 1 must be an int")
	}
	pair, err := objects.IterToSlice(args[1])
	if err != nil || len(pair) != 2 {
		return nil, objects.Raise(objects.ValueError, "expected a tuple of 2 integers")
	}
	soft, ok1 := objects.AsInt(pair[0])
	hard, ok2 := objects.AsInt(pair[1])
	if !ok1 || !ok2 {
		return nil, objects.Raise(objects.ValueError, "expected a tuple of 2 integers")
	}
	lim := syscall.Rlimit{Cur: uint64(soft), Max: uint64(hard)}
	if err := syscall.Setrlimit(int(which), &lim); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// resourceGetpagesize is resource.getpagesize(): the system page size in bytes.
func resourceGetpagesize(args []objects.Object) (objects.Object, error) {
	return objects.NewInt(int64(syscall.Getpagesize())), nil
}
