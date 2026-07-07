package runtime

import (
	"os"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// os.path is the path-manipulation half of os. In CPython it is pure Python
// (posixpath), which under the stdlib strategy would compile from the pinned
// tree; at floor scope it is provided in Go behind the os.path import so the
// early slices that need join, split and normpath do not wait on the whole
// pure-Python floor. The semantics follow posixpath: sep is '/', there is no
// altsep, and the routines here are the string-manipulation core plus the three
// stat probes exists, isfile and isdir.

func init() {
	moduleTable["os.path"] = &moduleEntry{builtin: true, exec: initOSPath}
}

func initOSPath(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("sep", objects.NewStr("/")); err != nil {
		return err
	}
	if err := set("altsep", objects.None); err != nil {
		return err
	}
	if err := set("extsep", objects.NewStr(".")); err != nil {
		return err
	}
	if err := set("pathsep", objects.NewStr(":")); err != nil {
		return err
	}
	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"join", pathJoin},
		{"split", pathSplit},
		{"splitext", pathSplitext},
		{"basename", pathBasename},
		{"dirname", pathDirname},
		{"isabs", pathIsabs},
		{"normpath", pathNormpath},
		{"abspath", pathAbspath},
		{"commonprefix", pathCommonprefix},
		{"expanduser", pathExpanduser},
		{"exists", pathExists},
		{"isfile", pathIsfile},
		{"isdir", pathIsdir},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// pathArg pulls the single string argument common to most os.path routines,
// raising the TypeError CPython gives for a non-str path.
func pathArg(args []objects.Object, fn string) (string, error) {
	if len(args) != 1 {
		return "", objects.Raise(objects.TypeError, "%s() takes exactly one argument (%d given)", fn, len(args))
	}
	s, ok := objects.AsStr(args[0])
	if !ok {
		return "", objects.Raise(objects.TypeError, "%s: path should be string, not %s", fn, args[0].TypeName())
	}
	return s, nil
}

// pathJoin joins path components the posixpath way: an absolute later component
// resets the result, and a separator is inserted only where one is missing.
func pathJoin(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 {
		return nil, objects.Raise(objects.TypeError, "join() missing 1 required positional argument: 'a'")
	}
	parts := make([]string, len(args))
	for i, a := range args {
		s, ok := objects.AsStr(a)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "join() argument must be str, not %s", a.TypeName())
		}
		parts[i] = s
	}
	path := parts[0]
	for _, b := range parts[1:] {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return objects.NewStr(path), nil
}

// splitHead splits at the last separator and trims trailing separators off the
// head unless the head is all separators, the shared core of split and dirname.
func splitHead(p string) (head, tail string) {
	i := strings.LastIndexByte(p, '/') + 1
	head, tail = p[:i], p[i:]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

func pathSplit(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "split")
	if err != nil {
		return nil, err
	}
	head, tail := splitHead(p)
	return objects.NewTuple([]objects.Object{objects.NewStr(head), objects.NewStr(tail)}), nil
}

func pathBasename(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "basename")
	if err != nil {
		return nil, err
	}
	i := strings.LastIndexByte(p, '/') + 1
	return objects.NewStr(p[i:]), nil
}

func pathDirname(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "dirname")
	if err != nil {
		return nil, err
	}
	head, _ := splitHead(p)
	return objects.NewStr(head), nil
}

// pathSplitext splits off the extension, following genericpath: the dot must
// come after the last separator and after at least one non-dot character, so a
// leading-dot name like .bashrc has no extension.
func pathSplitext(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "splitext")
	if err != nil {
		return nil, err
	}
	sepIndex := strings.LastIndexByte(p, '/')
	dotIndex := strings.LastIndexByte(p, '.')
	if dotIndex > sepIndex {
		filenameIndex := sepIndex + 1
		for filenameIndex < dotIndex {
			if p[filenameIndex] != '.' {
				return objects.NewTuple([]objects.Object{objects.NewStr(p[:dotIndex]), objects.NewStr(p[dotIndex:])}), nil
			}
			filenameIndex++
		}
	}
	return objects.NewTuple([]objects.Object{objects.NewStr(p), objects.NewStr("")}), nil
}

func pathIsabs(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "isabs")
	if err != nil {
		return nil, err
	}
	return objects.NewBool(strings.HasPrefix(p, "/")), nil
}

// normpath collapses redundant separators and up-level references, following
// posixpath: an empty path is ".", and exactly two leading slashes are
// preserved while three or more collapse to one.
func normpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initialSlashes = 2
		}
	}
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

func pathNormpath(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "normpath")
	if err != nil {
		return nil, err
	}
	return objects.NewStr(normpath(p)), nil
}

// pathAbspath resolves a path against the current directory and normalizes it.
func pathAbspath(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "abspath")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(p, "/") {
		wd, werr := os.Getwd()
		if werr != nil {
			return nil, objects.Raise("OSError", "%s", werr.Error())
		}
		if p == "" {
			p = wd
		} else {
			p = wd + "/" + p
		}
	}
	return objects.NewStr(normpath(p)), nil
}

// pathCommonprefix returns the longest common leading string, character by
// character, not path-aware, matching genericpath.commonprefix.
func pathCommonprefix(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "commonprefix() takes exactly one argument (%d given)", len(args))
	}
	it, err := objects.Iter(args[0])
	if err != nil {
		return nil, err
	}
	var items []string
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		s, ok := objects.AsStr(v)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "sequence item: expected str, %s found", v.TypeName())
		}
		items = append(items, s)
	}
	if len(items) == 0 {
		return objects.NewStr(""), nil
	}
	minS, maxS := items[0], items[0]
	for _, s := range items[1:] {
		if s < minS {
			minS = s
		}
		if s > maxS {
			maxS = s
		}
	}
	i := 0
	for i < len(minS) && i < len(maxS) && minS[i] == maxS[i] {
		i++
	}
	return objects.NewStr(minS[:i]), nil
}

// pathExpanduser replaces a leading ~ with the user's home directory, taken
// from the HOME environment variable; an unset HOME leaves the path unchanged.
func pathExpanduser(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "expanduser")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(p, "~") {
		return objects.NewStr(p), nil
	}
	i := strings.IndexByte(p, '/')
	if i < 0 {
		i = len(p)
	}
	if i != 1 {
		// ~user form is not resolved here; leave it untouched.
		return objects.NewStr(p), nil
	}
	home, ok := os.LookupEnv("HOME")
	if !ok || home == "" {
		return objects.NewStr(p), nil
	}
	home = strings.TrimRight(home, "/")
	return objects.NewStr(home + p[i:]), nil
}

func pathExists(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "exists")
	if err != nil {
		return nil, err
	}
	_, serr := os.Stat(p)
	return objects.NewBool(serr == nil), nil
}

func pathIsfile(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "isfile")
	if err != nil {
		return nil, err
	}
	info, serr := os.Stat(p)
	return objects.NewBool(serr == nil && info.Mode().IsRegular()), nil
}

func pathIsdir(args []objects.Object) (objects.Object, error) {
	p, err := pathArg(args, "isdir")
	if err != nil {
		return nil, err
	}
	info, serr := os.Stat(p)
	return objects.NewBool(serr == nil && info.IsDir()), nil
}
