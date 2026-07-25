package runtime

import (
	"os"
	"slices"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _colorize is CPython's internal helper for terminal color output: ANSI color
// constants, a can_colorize() probe, and the experimental theming support
// (Theme plus the Argparse/Syntax/Traceback/Unittest sections). traceback,
// logging, unittest, pydoc, argparse and code all `import _colorize` at import
// time, so its absence blocked that whole cluster.
//
// The pure _colorize.py cannot be vendored as-is: at module scope it builds its
// theme sections with `@dataclass`, whose generated __init__ runs through
// `exec` of source text, which an AOT runtime has no interpreter for
// (dataclasses.py:506). Every one of those imports failed at the identical spot.
//
// This native module hand-writes exactly what those dataclasses would produce:
// each section is an ordinary class with its color fields as instance
// attributes, mapping access (__getitem__/__len__/__iter__), copy_with, and a
// no_colors classmethod; Theme composes the four sections the same way. That
// sidesteps the exec wall without implementing dataclass at all. The module is
// portable, so it registers on every target.

// colorizeANSI is the ANSIColors constant table, name -> escape code, in
// definition order.
var colorizeANSI = []struct{ name, code string }{
	{"RESET", "\x1b[0m"},
	{"BLACK", "\x1b[30m"},
	{"BLUE", "\x1b[34m"},
	{"CYAN", "\x1b[36m"},
	{"GREEN", "\x1b[32m"},
	{"GREY", "\x1b[90m"},
	{"MAGENTA", "\x1b[35m"},
	{"RED", "\x1b[31m"},
	{"WHITE", "\x1b[37m"},
	{"YELLOW", "\x1b[33m"},
	{"BOLD", "\x1b[1m"},
	{"BOLD_BLACK", "\x1b[1;30m"},
	{"BOLD_BLUE", "\x1b[1;34m"},
	{"BOLD_CYAN", "\x1b[1;36m"},
	{"BOLD_GREEN", "\x1b[1;32m"},
	{"BOLD_MAGENTA", "\x1b[1;35m"},
	{"BOLD_RED", "\x1b[1;31m"},
	{"BOLD_WHITE", "\x1b[1;37m"},
	{"BOLD_YELLOW", "\x1b[1;33m"},
	{"INTENSE_BLACK", "\x1b[90m"},
	{"INTENSE_BLUE", "\x1b[94m"},
	{"INTENSE_CYAN", "\x1b[96m"},
	{"INTENSE_GREEN", "\x1b[92m"},
	{"INTENSE_MAGENTA", "\x1b[95m"},
	{"INTENSE_RED", "\x1b[91m"},
	{"INTENSE_WHITE", "\x1b[97m"},
	{"INTENSE_YELLOW", "\x1b[93m"},
	{"BACKGROUND_BLACK", "\x1b[40m"},
	{"BACKGROUND_BLUE", "\x1b[44m"},
	{"BACKGROUND_CYAN", "\x1b[46m"},
	{"BACKGROUND_GREEN", "\x1b[42m"},
	{"BACKGROUND_MAGENTA", "\x1b[45m"},
	{"BACKGROUND_RED", "\x1b[41m"},
	{"BACKGROUND_WHITE", "\x1b[47m"},
	{"BACKGROUND_YELLOW", "\x1b[43m"},
	{"INTENSE_BACKGROUND_BLACK", "\x1b[100m"},
	{"INTENSE_BACKGROUND_BLUE", "\x1b[104m"},
	{"INTENSE_BACKGROUND_CYAN", "\x1b[106m"},
	{"INTENSE_BACKGROUND_GREEN", "\x1b[102m"},
	{"INTENSE_BACKGROUND_MAGENTA", "\x1b[105m"},
	{"INTENSE_BACKGROUND_RED", "\x1b[101m"},
	{"INTENSE_BACKGROUND_WHITE", "\x1b[107m"},
	{"INTENSE_BACKGROUND_YELLOW", "\x1b[103m"},
}

// colorizeSection describes one theme section: its class name and its ordered
// fields with their default color codes.
type colorizeSection struct {
	name   string
	fields []struct{ field, def string }
}

func c(f, d string) struct{ field, def string } { return struct{ field, def string }{f, d} }

// The four sections, matching _colorize.py exactly. ANSI codes are inlined by
// value so the defaults do not depend on the ANSIColors object.
var colorizeSections = []colorizeSection{
	{"Argparse", []struct{ field, def string }{
		c("usage", "\x1b[1;34m"), c("prog", "\x1b[1;35m"), c("prog_extra", "\x1b[35m"),
		c("heading", "\x1b[1;34m"), c("summary_long_option", "\x1b[36m"),
		c("summary_short_option", "\x1b[32m"), c("summary_label", "\x1b[33m"),
		c("summary_action", "\x1b[32m"), c("long_option", "\x1b[1;36m"),
		c("short_option", "\x1b[1;32m"), c("label", "\x1b[1;33m"),
		c("action", "\x1b[1;32m"), c("reset", "\x1b[0m"),
	}},
	{"Syntax", []struct{ field, def string }{
		c("prompt", "\x1b[1;35m"), c("keyword", "\x1b[1;34m"), c("keyword_constant", "\x1b[1;34m"),
		c("builtin", "\x1b[36m"), c("comment", "\x1b[31m"), c("string", "\x1b[32m"),
		c("number", "\x1b[33m"), c("op", "\x1b[0m"), c("definition", "\x1b[1m"),
		c("soft_keyword", "\x1b[1;34m"), c("reset", "\x1b[0m"),
	}},
	{"Traceback", []struct{ field, def string }{
		c("type", "\x1b[1;35m"), c("message", "\x1b[35m"), c("filename", "\x1b[35m"),
		c("line_no", "\x1b[35m"), c("frame", "\x1b[35m"), c("error_highlight", "\x1b[1;31m"),
		c("error_range", "\x1b[31m"), c("reset", "\x1b[0m"),
	}},
	{"Unittest", []struct{ field, def string }{
		c("passed", "\x1b[32m"), c("warn", "\x1b[33m"), c("fail", "\x1b[31m"),
		c("fail_info", "\x1b[1;31m"), c("reset", "\x1b[0m"),
	}},
}

// colorizeTheme holds the currently set theme (set_theme), default_theme until
// changed.
var colorizeCurrentTheme objects.Object

func init() {
	moduleTable["_colorize"] = &moduleEntry{builtin: true, exec: initColorize}
}

func initColorize(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	if err := set("COLORIZE", objects.True); err != nil {
		return err
	}

	// ANSIColors: a class carrying the color constants as class attributes. An
	// instance reads them through the class. NoColors is an instance whose
	// per-instance attributes shadow every constant with "".
	ansiNames := make([]string, len(colorizeANSI))
	ansiVals := make([]objects.Object, len(colorizeANSI))
	codeSet := make([]objects.Object, 0, len(colorizeANSI))
	for i, a := range colorizeANSI {
		ansiNames[i] = a.name
		ansiVals[i] = objects.NewStr(a.code)
		codeSet = append(codeSet, objects.NewStr(a.code))
	}
	ansiClass, err := objects.NewClass("ANSIColors", "ANSIColors", nil, ansiNames, ansiVals, nil, nil)
	if err != nil {
		return err
	}
	if err := set("ANSIColors", ansiClass); err != nil {
		return err
	}
	noColors, err := objects.Call(ansiClass, nil)
	if err != nil {
		return err
	}
	for _, a := range colorizeANSI {
		if err := objects.StoreAttr(noColors, a.name, objects.NewStr("")); err != nil {
			return err
		}
	}
	if err := set("NoColors", noColors); err != nil {
		return err
	}
	colorCodes, err := objects.NewSet(codeSet)
	if err != nil {
		return err
	}
	if err := set("ColorCodes", colorCodes); err != nil {
		return err
	}

	// The four theme sections and the Theme that composes them.
	sectionClasses := map[string]objects.Object{}
	for _, spec := range colorizeSections {
		cls, err := buildColorizeSection(spec)
		if err != nil {
			return err
		}
		sectionClasses[spec.name] = cls
		if err := set(spec.name, cls); err != nil {
			return err
		}
	}
	themeClass, err := buildColorizeTheme(sectionClasses)
	if err != nil {
		return err
	}
	if err := set("Theme", themeClass); err != nil {
		return err
	}

	// default_theme = Theme(); theme_no_color = default_theme.no_colors().
	defaultTheme, err := objects.Call(themeClass, nil)
	if err != nil {
		return err
	}
	if err := set("default_theme", defaultTheme); err != nil {
		return err
	}
	noColorFn, err := objects.LoadAttr(defaultTheme, "no_colors")
	if err != nil {
		return err
	}
	themeNoColor, err := objects.Call(noColorFn, nil)
	if err != nil {
		return err
	}
	if err := set("theme_no_color", themeNoColor); err != nil {
		return err
	}
	colorizeCurrentTheme = defaultTheme

	// Module functions.
	if err := set("can_colorize", objects.NewFuncKw("can_colorize", colorizeCanColorizeKw)); err != nil {
		return err
	}
	if err := set("decolor", objects.NewFunc("decolor", 1, func(args []objects.Object) (objects.Object, error) {
		return colorizeDecolor(args, colorCodes)
	})); err != nil {
		return err
	}
	if err := set("get_colors", objects.NewFuncKw("get_colors", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		return colorizeGetColors(pos, kwNames, kwVals, ansiClass, noColors)
	})); err != nil {
		return err
	}
	if err := set("get_theme", objects.NewFuncKw("get_theme", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		return colorizeGetTheme(pos, kwNames, kwVals, themeNoColor)
	})); err != nil {
		return err
	}
	if err := set("set_theme", objects.NewFunc("set_theme", 1, func(args []objects.Object) (objects.Object, error) {
		return colorizeSetTheme(args)
	})); err != nil {
		return err
	}
	return nil
}

// buildColorizeSection builds one theme-section class. Its methods close over
// the field list and the class value (assigned after NewClass) so copy_with and
// no_colors can construct a fresh instance of the same type.
func buildColorizeSection(spec colorizeSection) (objects.Object, error) {
	fields := make([]string, len(spec.fields))
	defs := make([]string, len(spec.fields))
	for i, f := range spec.fields {
		fields[i] = f.field
		defs[i] = f.def
	}
	var cls objects.Object

	initFn := objects.NewMethodKw("__init__", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "__init__ needs self")
		}
		self := pos[0]
		kw := colorizeKwMap(kwNames, kwVals)
		for i, name := range fields {
			v, ok := kw[name]
			if !ok {
				v = objects.NewStr(defs[i])
			}
			if err := objects.StoreAttr(self, name, v); err != nil {
				return nil, err
			}
		}
		return objects.None, nil
	})

	getItem := objects.NewMethod("__getitem__", 2, func(args []objects.Object) (objects.Object, error) {
		key, ok := objects.AsStr(args[1])
		if !ok {
			return nil, objects.Raise(objects.KeyError, "%v", args[1])
		}
		if slices.Contains(fields, key) {
			return objects.LoadAttr(args[0], key)
		}
		return nil, objects.Raise(objects.KeyError, "'%s'", key)
	})

	lenFn := objects.NewMethod("__len__", 1, func(args []objects.Object) (objects.Object, error) {
		return objects.NewInt(int64(len(fields))), nil
	})

	iterFn := objects.NewMethod("__iter__", 1, func(args []objects.Object) (objects.Object, error) {
		elts := make([]objects.Object, len(fields))
		for i, name := range fields {
			elts[i] = objects.NewStr(name)
		}
		return objects.CallMethod(objects.NewList(elts), "__iter__", nil)
	})

	copyWith := objects.NewMethodKw("copy_with", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		self := pos[0]
		kw := colorizeKwMap(kwNames, kwVals)
		vals := make([]objects.Object, len(fields))
		for i, name := range fields {
			if v, ok := kw[name]; ok {
				vals[i] = v
				continue
			}
			cur, err := objects.LoadAttr(self, name)
			if err != nil {
				return nil, err
			}
			vals[i] = cur
		}
		return objects.CallKw(cls, nil, fields, vals)
	})

	noColors := objects.NewClassMethod(objects.NewFunc("no_colors", -1, func(args []objects.Object) (objects.Object, error) {
		vals := make([]objects.Object, len(fields))
		empty := objects.NewStr("")
		for i := range fields {
			vals[i] = empty
		}
		return objects.CallKw(cls, nil, fields, vals)
	}))

	names := []string{"__init__", "__getitem__", "__len__", "__iter__", "copy_with", "no_colors"}
	vals := []objects.Object{initFn, getItem, lenFn, iterFn, copyWith, noColors}
	built, err := objects.NewClass(spec.name, "_colorize."+spec.name, nil, names, vals, nil, nil)
	if err != nil {
		return nil, err
	}
	cls = built
	return built, nil
}

// buildColorizeTheme builds the Theme class over the four section classes.
func buildColorizeTheme(sections map[string]objects.Object) (objects.Object, error) {
	order := []string{"argparse", "syntax", "traceback", "unittest"}
	classOf := map[string]objects.Object{
		"argparse":  sections["Argparse"],
		"syntax":    sections["Syntax"],
		"traceback": sections["Traceback"],
		"unittest":  sections["Unittest"],
	}
	var cls objects.Object

	initFn := objects.NewMethodKw("__init__", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "__init__ needs self")
		}
		self := pos[0]
		kw := colorizeKwMap(kwNames, kwVals)
		for _, name := range order {
			v, ok := kw[name]
			if !ok {
				// default_factory: a fresh default section.
				fresh, err := objects.Call(classOf[name], nil)
				if err != nil {
					return nil, err
				}
				v = fresh
			}
			if err := objects.StoreAttr(self, name, v); err != nil {
				return nil, err
			}
		}
		return objects.None, nil
	})

	copyWith := objects.NewMethodKw("copy_with", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		self := pos[0]
		kw := colorizeKwMap(kwNames, kwVals)
		vals := make([]objects.Object, len(order))
		for i, name := range order {
			// `section or self.section`: a None override falls back to the current.
			if v, ok := kw[name]; ok && v != objects.None {
				vals[i] = v
				continue
			}
			cur, err := objects.LoadAttr(self, name)
			if err != nil {
				return nil, err
			}
			vals[i] = cur
		}
		return objects.CallKw(cls, nil, order, vals)
	})

	noColors := objects.NewClassMethod(objects.NewFunc("no_colors", -1, func(args []objects.Object) (objects.Object, error) {
		vals := make([]objects.Object, len(order))
		for i, name := range order {
			fn, err := objects.LoadAttr(classOf[name], "no_colors")
			if err != nil {
				return nil, err
			}
			sec, err := objects.Call(fn, nil)
			if err != nil {
				return nil, err
			}
			vals[i] = sec
		}
		return objects.CallKw(cls, nil, order, vals)
	}))

	names := []string{"__init__", "copy_with", "no_colors"}
	vals := []objects.Object{initFn, copyWith, noColors}
	built, err := objects.NewClass("Theme", "_colorize.Theme", nil, names, vals, nil, nil)
	if err != nil {
		return nil, err
	}
	cls = built
	return built, nil
}

func colorizeKwMap(kwNames []string, kwVals []objects.Object) map[string]objects.Object {
	m := make(map[string]objects.Object, len(kwNames))
	for i, n := range kwNames {
		m[n] = kwVals[i]
	}
	return m
}

// colorizeCanColorizeKw is can_colorize(*, file=None). It honors the same
// environment controls CPython does, then falls back to whether the target
// stream is a terminal.
func colorizeCanColorizeKw(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	var file objects.Object = objects.None
	for i, n := range kwNames {
		if n == "file" {
			file = kwVals[i]
		}
	}
	switch os.Getenv("PYTHON_COLORS") {
	case "0":
		return objects.False, nil
	case "1":
		return objects.True, nil
	}
	if os.Getenv("NO_COLOR") != "" {
		return objects.False, nil
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return objects.True, nil
	}
	if os.Getenv("TERM") == "dumb" {
		return objects.False, nil
	}
	// No file given means sys.stdout; check the process stdout directly.
	if file == objects.None {
		return objects.NewBool(colorizeIsTerminal(os.Stdout)), nil
	}
	// A file object: it must expose fileno() to be colorizable, and that
	// descriptor must be a tty. A file without fileno (e.g. StringIO) is not.
	fnAttr, err := objects.LoadAttr(file, "fileno")
	if err != nil {
		return objects.False, nil
	}
	fdObj, err := objects.Call(fnAttr, nil)
	if err != nil {
		// Fall back to file.isatty() the way the pure code does on OSError.
		if isatty, err2 := objects.LoadAttr(file, "isatty"); err2 == nil {
			if r, err3 := objects.Call(isatty, nil); err3 == nil {
				t, _ := objects.TruthOf(r)
				return objects.NewBool(t), nil
			}
		}
		return objects.False, nil
	}
	fd, ok := objects.AsInt(fdObj)
	if !ok {
		return objects.False, nil
	}
	return objects.NewBool(colorizeIsTerminal(os.NewFile(uintptr(fd), ""))), nil
}

// colorizeIsTerminal reports whether f is a character device (a terminal),
// using only os.Stat, so it stays stdlib-only and portable.
func colorizeIsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func colorizeDecolor(args []objects.Object, colorCodes objects.Object) (objects.Object, error) {
	text, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "decolor() argument must be str")
	}
	codes, err := objects.IterToSlice(colorCodes)
	if err != nil {
		return nil, err
	}
	for _, cObj := range codes {
		code, ok := objects.AsStr(cObj)
		if !ok {
			continue
		}
		text = strings.ReplaceAll(text, code, "")
	}
	return objects.NewStr(text), nil
}

func colorizeGetColors(pos []objects.Object, kwNames []string, kwVals []objects.Object, ansiClass, noColors objects.Object) (objects.Object, error) {
	colorize := false
	if len(pos) >= 1 {
		t, err := objects.TruthOf(pos[0])
		if err != nil {
			return nil, err
		}
		colorize = t
	}
	var file objects.Object = objects.None
	for i, n := range kwNames {
		switch n {
		case "colorize":
			t, err := objects.TruthOf(kwVals[i])
			if err != nil {
				return nil, err
			}
			colorize = t
		case "file":
			file = kwVals[i]
		}
	}
	if !colorize {
		can, err := colorizeCanColorizeKw(nil, []string{"file"}, []objects.Object{file})
		if err != nil {
			return nil, err
		}
		t, _ := objects.TruthOf(can)
		colorize = t
	}
	if colorize {
		return objects.Call(ansiClass, nil)
	}
	return noColors, nil
}

func colorizeGetTheme(_ []objects.Object, kwNames []string, kwVals []objects.Object, themeNoColor objects.Object) (objects.Object, error) {
	var ttyFile objects.Object = objects.None
	forceColor, forceNoColor := false, false
	for i, n := range kwNames {
		switch n {
		case "tty_file":
			ttyFile = kwVals[i]
		case "force_color":
			t, err := objects.TruthOf(kwVals[i])
			if err != nil {
				return nil, err
			}
			forceColor = t
		case "force_no_color":
			t, err := objects.TruthOf(kwVals[i])
			if err != nil {
				return nil, err
			}
			forceNoColor = t
		}
	}
	if forceColor {
		return colorizeCurrentTheme, nil
	}
	if !forceNoColor {
		can, err := colorizeCanColorizeKw(nil, []string{"file"}, []objects.Object{ttyFile})
		if err != nil {
			return nil, err
		}
		if t, _ := objects.TruthOf(can); t {
			return colorizeCurrentTheme, nil
		}
	}
	return themeNoColor, nil
}

func colorizeSetTheme(args []objects.Object) (objects.Object, error) {
	t := args[0]
	if t.TypeName() != "Theme" {
		return nil, objects.Raise(objects.ValueError, "Expected Theme object, found %s", objects.Str(t))
	}
	colorizeCurrentTheme = t
	return objects.None, nil
}
