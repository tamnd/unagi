package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// initColorizeModule builds a fresh _colorize module for a test.
func initColorizeModule(t *testing.T) objects.Object {
	t.Helper()
	m := objects.NewModule("_colorize", "<_colorize>")
	if err := initColorize(m); err != nil {
		t.Fatalf("initColorize: %v", err)
	}
	return m
}

// TestColorizeTheme checks the theme sections behave like the dataclasses they
// stand in for: mapping access, copy_with keyword override without mutating the
// original, and a no_colors variant whose every role is empty.
func TestColorizeTheme(t *testing.T) {
	m := initColorizeModule(t)

	call := func(name string, args ...objects.Object) objects.Object {
		t.Helper()
		f, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		r, err := objects.Call(f, args)
		if err != nil {
			t.Fatalf("call %s: %v", name, err)
		}
		return r
	}
	callKw := func(name string, kwNames []string, kwVals []objects.Object) objects.Object {
		t.Helper()
		f, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		r, err := objects.CallKw(f, nil, kwNames, kwVals)
		if err != nil {
			t.Fatalf("callKw %s: %v", name, err)
		}
		return r
	}

	// force_no_color yields a theme whose codes are all empty.
	theme := callKw("get_theme", []string{"force_no_color"}, []objects.Object{objects.True})
	syntax, err := objects.LoadAttr(theme, "syntax")
	if err != nil {
		t.Fatalf("LoadAttr syntax: %v", err)
	}
	if comment, _ := objects.AsStr(mustLoad(t, syntax, "comment")); comment != "" {
		t.Fatalf("force_no_color comment = %q, want empty", comment)
	}

	// force_color yields non-empty codes; copy_with overrides just one role.
	themeC := callKw("get_theme", []string{"force_color"}, []objects.Object{objects.True})
	sc := mustLoad(t, themeC, "syntax")
	origComment, _ := objects.AsStr(mustLoad(t, sc, "comment"))
	if origComment == "" {
		t.Fatalf("force_color comment is empty")
	}
	// __getitem__ agrees with attribute access; unknown role is a KeyError.
	if got, _ := objects.AsStr(mustGetItem(t, sc, "comment")); got != origComment {
		t.Fatalf("sc['comment'] = %q, want %q", got, origComment)
	}
	if _, err := objects.GetItem(sc, objects.NewStr("nope")); err == nil {
		t.Fatalf("sc['nope'] did not raise")
	}

	// copy_with with a keyword must reach the bound native method (this is the
	// path that a positional-only instance binding would have broken).
	s2, err := objects.CallKw(mustLoad(t, sc, "copy_with"), nil, []string{"comment"}, []objects.Object{objects.NewStr("X")})
	if err != nil {
		t.Fatalf("copy_with(comment=X): %v", err)
	}
	if got, _ := objects.AsStr(mustLoad(t, s2, "comment")); got != "X" {
		t.Fatalf("copy_with comment = %q, want X", got)
	}
	if got, _ := objects.AsStr(mustLoad(t, sc, "comment")); got != origComment {
		t.Fatalf("copy_with mutated original: %q", got)
	}

	// no_colors returns a same-typed section with every role empty.
	nc, err := objects.Call(mustLoad(t, sc, "no_colors"), nil)
	if err != nil {
		t.Fatalf("no_colors: %v", err)
	}
	if nc.TypeName() != sc.TypeName() {
		t.Fatalf("no_colors type = %s, want %s", nc.TypeName(), sc.TypeName())
	}
	roles, err := objects.IterToSlice(nc)
	if err != nil {
		t.Fatalf("iter no_colors: %v", err)
	}
	if len(roles) == 0 {
		t.Fatalf("no_colors yielded no roles")
	}
	for _, role := range roles {
		v, err := objects.GetItem(nc, role)
		if err != nil {
			t.Fatalf("GetItem %v: %v", role, err)
		}
		if s, _ := objects.AsStr(v); s != "" {
			t.Fatalf("no_colors role %v = %q, want empty", role, s)
		}
	}

	// decolor strips ANSI escapes.
	if s, _ := objects.AsStr(call("decolor", objects.NewStr("\x1b[35mhi\x1b[0m"))); s != "hi" {
		t.Fatalf("decolor = %q, want hi", s)
	}

	// set_theme type-checks: a non-Theme is a ValueError.
	setTheme, _ := objects.LoadAttr(m, "set_theme")
	if _, err := objects.Call(setTheme, []objects.Object{objects.NewStr("nope")}); err == nil {
		t.Fatalf("set_theme('nope') did not raise")
	}
	if _, err := objects.Call(setTheme, []objects.Object{theme}); err != nil {
		t.Fatalf("set_theme(theme): %v", err)
	}
}

// TestColorizeCanColorizeEnv checks can_colorize honors NO_COLOR / FORCE_COLOR.
func TestColorizeCanColorizeEnv(t *testing.T) {
	m := initColorizeModule(t)
	canColorize, err := objects.LoadAttr(m, "can_colorize")
	if err != nil {
		t.Fatalf("LoadAttr can_colorize: %v", err)
	}

	t.Setenv("NO_COLOR", "1")
	r, err := objects.Call(canColorize, nil)
	if err != nil {
		t.Fatalf("can_colorize: %v", err)
	}
	if r != objects.False {
		t.Fatalf("NO_COLOR can_colorize = %v, want False", r)
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")
	r, err = objects.Call(canColorize, nil)
	if err != nil {
		t.Fatalf("can_colorize: %v", err)
	}
	if r != objects.True {
		t.Fatalf("FORCE_COLOR can_colorize = %v, want True", r)
	}
}

func mustLoad(t *testing.T, o objects.Object, name string) objects.Object {
	t.Helper()
	v, err := objects.LoadAttr(o, name)
	if err != nil {
		t.Fatalf("LoadAttr %s: %v", name, err)
	}
	return v
}

func mustGetItem(t *testing.T, o objects.Object, key string) objects.Object {
	t.Helper()
	v, err := objects.GetItem(o, objects.NewStr(key))
	if err != nil {
		t.Fatalf("GetItem %s: %v", key, err)
	}
	return v
}
