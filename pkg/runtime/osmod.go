package runtime

import (
	"os"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// os is a built-in module at portable-core scope: the process and environment
// surface a small program reaches for, provided in Go behind the os import the
// way sys, time and io are. The path-manipulation half lives in os.path, a
// later slice. The platform constants are the POSIX values, which is what both
// supported hosts report; a Windows build would carry the nt values.

func init() {
	moduleTable["os"] = &moduleEntry{builtin: true, exec: initOS}
}

func initOS(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	consts := []struct {
		name string
		val  string
	}{
		{"name", "posix"},
		{"sep", "/"},
		{"linesep", "\n"},
		{"pathsep", ":"},
		{"curdir", "."},
		{"pardir", ".."},
		{"extsep", "."},
		{"devnull", "/dev/null"},
		{"defpath", ":/bin:/usr/bin"},
	}
	for _, c := range consts {
		if err := set(c.name, objects.NewStr(c.val)); err != nil {
			return err
		}
	}
	// altsep is the separator a second path syntax uses; POSIX has none, so it
	// is None rather than a string.
	if err := set("altsep", objects.None); err != nil {
		return err
	}

	// environ is a live dict seeded from the process environment. Reads, writes,
	// membership and .get all work through the dict, which is the surface floor
	// code uses; getenv reads the same dict so a write is visible through both.
	environ, err := objects.NewDict(nil, nil)
	if err != nil {
		return err
	}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			if serr := objects.SetItem(environ, objects.NewStr(kv[:i]), objects.NewStr(kv[i+1:])); serr != nil {
				return serr
			}
		}
	}
	if err := set("environ", environ); err != nil {
		return err
	}

	getenv := func(args []objects.Object) (objects.Object, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, objects.Raise(objects.TypeError, "getenv() takes 1 or 2 arguments (%d given)", len(args))
		}
		def := objects.Object(objects.None)
		if len(args) == 2 {
			def = args[1]
		}
		v, err := objects.GetItem(environ, args[0])
		if err != nil {
			// The only error a str key raises here is the missing-key KeyError,
			// which getenv turns into the default.
			return def, nil
		}
		return v, nil
	}
	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"getenv", getenv},
		{"getcwd", osGetcwd},
		{"getpid", osGetpid},
		{"listdir", osListdir},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

func osGetcwd(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getcwd() takes no arguments (%d given)", len(args))
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return objects.NewStr(wd), nil
}

func osGetpid(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getpid() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(os.Getpid())), nil
}

// osListdir lists the names in a directory, defaulting to the current one. The
// entries come back in name order so a program that does not sort still sees a
// stable listing.
func osListdir(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "listdir() takes at most 1 argument (%d given)", len(args))
	}
	dir := "."
	if len(args) == 1 && args[0] != objects.None {
		s, ok := objects.AsStr(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "listdir: path should be string, not %s", args[0].TypeName())
		}
		dir = s
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, objects.Raise("FileNotFoundError", "%s", err.Error())
	}
	names := make([]objects.Object, len(entries))
	for i, e := range entries {
		names[i] = objects.NewStr(e.Name())
	}
	return objects.NewList(names), nil
}
