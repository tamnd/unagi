package runtime

import (
	"os/user"
	"strconv"

	"github.com/tamnd/unagi/pkg/objects"
)

// pwd is the password-database accelerator posixpath.expanduser reaches for to
// resolve ~ and ~user: it looks a home directory up by name (getpwnam) or by
// the process uid (getpwuid). The lookups run through the standard os/user
// package, which reads the host user database the same way CPython's getpwnam(3)
// does (the real directory service on darwin, /etc/passwd on Linux), so a
// program sees the same home directory the oracle does.
//
// os/user surfaces the name, uid, gid, gecos and home directory but not the
// shell or the (shadowed) password field, so pw_passwd is the conventional "*"
// mask and pw_shell is empty. Nothing in the floor reads those two, and their
// real values are host-specific, so no fixture asserts them; the fields exist so
// struct_passwd keeps CPython's shape and index order.

func init() {
	moduleTable["pwd"] = &moduleEntry{builtin: true, exec: initPwd}
}

// pwdStructPasswd is the structseq class getpwnam/getpwuid return. All seven
// fields are both named and in the sequence, in CPython's order, so the value
// unpacks as a 7-tuple and answers pw_name/pw_dir/... alike.
var pwdStructPasswd = objects.NewStructSeqType(
	"struct_passwd", "pwd.struct_passwd",
	[]string{"pw_name", "pw_passwd", "pw_uid", "pw_gid", "pw_gecos", "pw_dir", "pw_shell"},
	7, 0,
)

func initPwd(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("struct_passwd", pwdStructPasswd); err != nil {
		return err
	}
	if err := set("getpwnam", objects.NewFunc("getpwnam", 1, pwdGetpwnam)); err != nil {
		return err
	}
	if err := set("getpwuid", objects.NewFunc("getpwuid", 1, pwdGetpwuid)); err != nil {
		return err
	}
	return nil
}

// pwdEntry turns an os/user record into a struct_passwd. The uid and gid come
// back as strings on POSIX and parse as ints.
func pwdEntry(u *user.User) objects.Object {
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	vals := []objects.Object{
		objects.NewStr(u.Username),
		objects.NewStr("*"),
		objects.NewInt(int64(uid)),
		objects.NewInt(int64(gid)),
		objects.NewStr(u.Name),
		objects.NewStr(u.HomeDir),
		objects.NewStr(""),
	}
	return pwdStructPasswd.NewStructSeq(vals, vals)
}

// pwdGetpwnam is pwd.getpwnam(name): the entry for a login name, or KeyError if
// the name is not in the database, with CPython's wording.
func pwdGetpwnam(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getpwnam() argument must be str, not %s", args[0].TypeName())
	}
	u, err := user.Lookup(name)
	if err != nil {
		return nil, objects.Raise("KeyError", "getpwnam(): name not found: '%s'", name)
	}
	return pwdEntry(u), nil
}

// pwdGetpwuid is pwd.getpwuid(uid): the entry for a numeric user id, or KeyError
// if the uid is not in the database, with CPython's wording.
func pwdGetpwuid(args []objects.Object) (objects.Object, error) {
	uid, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "getpwuid() argument must be int, not %s", args[0].TypeName())
	}
	u, err := user.LookupId(strconv.FormatInt(uid, 10))
	if err != nil {
		return nil, objects.Raise("KeyError", "getpwuid(): uid not found: %d", uid)
	}
	return pwdEntry(u), nil
}
