//go:build !windows

package runtime

import (
	"os/user"
	"strconv"

	"github.com/tamnd/unagi/pkg/objects"
)

// grp is the group-database accelerator, the sibling of pwd. subprocess reads a
// group's gid by name to drop privileges (grp.getgrnam(group).gr_gid) and
// tarfile maps a gid to a group name and back when it archives ownership. Both
// lookups run through the standard os/user package, which reads the host group
// database the same way CPython's getgrnam(3) does (the real directory service
// on darwin, /etc/group on Linux), so a program sees the same group the oracle
// does.
//
// os/user surfaces the group name and gid but not the (shadowed) password field
// or the member list, so gr_passwd is the conventional "*" mask and gr_mem is an
// empty list. The floor reads only gr_name and gr_gid, and the real values of
// the other two are host-specific, so no fixture asserts them; the fields exist
// so struct_group keeps CPython's shape and index order.
//
// getgrall is omitted, as pwd omits getpwall: os/user offers no enumeration, and
// a program that reaches for it gets a clean AttributeError rather than a
// wrong-empty list. Nothing in the floor calls it.

func init() {
	moduleTable["grp"] = &moduleEntry{builtin: true, exec: initGrp}
}

// grpStructGroup is the structseq class getgrnam/getgrgid return. All four
// fields are both named and in the sequence, in CPython's order, so the value
// unpacks as a 4-tuple and answers gr_name/gr_gid/... alike.
var grpStructGroup = objects.NewStructSeqType(
	"struct_group", "grp.struct_group",
	[]string{"gr_name", "gr_passwd", "gr_gid", "gr_mem"},
	4, 0,
)

func initGrp(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("struct_group", grpStructGroup); err != nil {
		return err
	}
	if err := set("getgrnam", objects.NewFunc("getgrnam", 1, grpGetgrnam)); err != nil {
		return err
	}
	if err := set("getgrgid", objects.NewFunc("getgrgid", 1, grpGetgrgid)); err != nil {
		return err
	}
	return nil
}

// grpEntry turns an os/user group record into a struct_group. The gid comes back
// as a string on POSIX and parses as an int.
func grpEntry(g *user.Group) objects.Object {
	gid, _ := strconv.Atoi(g.Gid)
	vals := []objects.Object{
		objects.NewStr(g.Name),
		objects.NewStr("*"),
		objects.NewInt(int64(gid)),
		objects.NewList(nil),
	}
	return grpStructGroup.NewStructSeq(vals, vals)
}

// grpGetgrnam is grp.getgrnam(name): the entry for a group name, or KeyError if
// the name is not in the database, with CPython's wording.
func grpGetgrnam(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getgrnam() argument must be str, not %s", args[0].TypeName())
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return nil, objects.Raise("KeyError", "getgrnam(): name not found: '%s'", name)
	}
	return grpEntry(g), nil
}

// grpGetgrgid is grp.getgrgid(gid): the entry for a numeric group id, or KeyError
// if the gid is not in the database, with CPython's wording.
func grpGetgrgid(args []objects.Object) (objects.Object, error) {
	gid, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getgrgid() argument must be int, not %s", args[0].TypeName())
	}
	g, err := user.LookupGroupId(strconv.FormatInt(gid, 10))
	if err != nil {
		return nil, objects.Raise("KeyError", "getgrgid(): gid not found: %d", gid)
	}
	return grpEntry(g), nil
}
