//go:build !windows

package runtime

import "github.com/tamnd/unagi/pkg/objects"

// posixUnameResultType is the structseq os.uname returns: the five utsname
// fields CPython names, every one both part of the visible sequence and readable
// by name. os.py re-exports the type through posix.__all__, so it surfaces as
// os.uname_result the way CPython exposes it. os.uname is posix-only (this file
// is not built on Windows), matching CPython where os.uname does not exist there.
var posixUnameResultType = objects.NewStructSeqType(
	"uname_result", "posix.uname_result",
	[]string{"sysname", "nodename", "release", "version", "machine"},
	5, 0,
)

// unameData is the host view of uname(3), filled by the per-GOOS unameFields
// helper: the utsname struct on Linux, sysctl reads on the BSD family (macOS
// included) where there is no uname syscall.
type unameData struct {
	sysname  string
	nodename string
	release  string
	version  string
	machine  string
}

// posixUname reports the host's uname fields the way CPython's os.uname does, a
// structseq of sysname, nodename, release, version and machine.
func posixUname(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "uname() takes no arguments (%d given)", len(args))
	}
	u, err := unameFields()
	if err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	vals := []objects.Object{
		objects.NewStr(u.sysname),
		objects.NewStr(u.nodename),
		objects.NewStr(u.release),
		objects.NewStr(u.version),
		objects.NewStr(u.machine),
	}
	return posixUnameResultType.NewStructSeq(vals, vals), nil
}
