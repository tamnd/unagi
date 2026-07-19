package objects

import "fmt"

// frameObject is one entry of the lightweight shadow call stack unagi keeps on
// each Thread so sys._getframe() has something to return. unagi compiles to Go
// and has no interpreter frames, so a compiled Python function pushes one of
// these on entry and pops it on exit; the frame carries the caller's module
// namespace (f_globals), a code object (co_filename/co_name), the current line
// and a link to the caller frame. It is deliberately thin: f_globals, f_code
// and f_back are faithful, and f_locals is a best-effort proxy over the module
// namespace (locals are not tracked in the compiled model, a documented
// divergence recorded in the frame log).
type frameObject struct {
	back      *frameObject
	code      *codeObject
	globals   Object // the *Module the frame runs in, or nil for a plain script
	line      int
	optimized bool // a function frame (fast locals) versus a module or class body
}

func (*frameObject) TypeName() string { return "frame" }

// SetLine updates the frame's current line, called from compiled code as
// execution advances so f_lineno reads back the live line.
func (f *frameObject) SetLine(n int) { f.line = n }

// codeObject is the code object a frame points at. Only the fields the stdlib
// reads through a frame are modeled (co_filename, co_name, co_qualname,
// co_firstlineno); the rest are an accepted divergence until a real code object
// lands.
type codeObject struct {
	filename  string
	name      string
	qualname  string
	firstline int
}

func (*codeObject) TypeName() string { return "code" }

// framelocalsproxyObject is the type of frame.f_locals on 3.13+. _collections_abc
// takes type(sys._getframe().f_locals) and registers it as a Mapping, so the
// type identity is what matters here; the mapping behavior over a frame is
// deferred (a documented divergence) since the floor only needs the type.
type framelocalsproxyObject struct {
	frame *frameObject
}

func (*framelocalsproxyObject) TypeName() string { return "FrameLocalsProxy" }

// NewFrame builds a frame for the shadow stack. back links to the caller frame,
// globals is the module the frame runs in (may be nil), file/name/qual/firstline
// seed the code object, and optimized marks a function frame (fast locals) apart
// from a module or class body, which decides whether f_locals is a proxy or the
// namespace dict.
func NewFrame(back *frameObject, globals Object, file, name, qual string, firstline int, optimized bool) *frameObject {
	return &frameObject{
		back:      back,
		globals:   globals,
		line:      firstline,
		optimized: optimized,
		code: &codeObject{
			filename:  file,
			name:      name,
			qualname:  qual,
			firstline: firstline,
		},
	}
}

// frameLoadAttr answers the frame attributes the stdlib reads. f_globals hands
// back the module namespace, f_locals a proxy over it, f_back the caller frame
// or None, f_lineno the live line and f_code the code object.
func frameLoadAttr(f *frameObject, name string) (Object, error) {
	switch name {
	case "f_back":
		if f.back == nil {
			return None, nil
		}
		return f.back, nil
	case "f_code":
		return f.code, nil
	case "f_globals":
		if f.globals == nil {
			d, _ := NewDict(nil, nil)
			return d, nil
		}
		return f.globals, nil
	case "f_locals":
		// A function frame exposes its fast locals through a FrameLocalsProxy on
		// 3.13+, the type _collections_abc registers as a Mapping. A module or
		// class body has no fast locals, so its f_locals is the namespace dict
		// itself, exactly as CPython hands back the real mapping there.
		if f.optimized {
			return &framelocalsproxyObject{frame: f}, nil
		}
		if f.globals == nil {
			d, _ := NewDict(nil, nil)
			return d, nil
		}
		return f.globals, nil
	case "f_lineno":
		return NewInt(int64(f.line)), nil
	case "f_builtins":
		// The builtins namespace is not modeled as a frame attribute yet; an
		// empty mapping keeps a read from crashing, a documented divergence.
		d, _ := NewDict(nil, nil)
		return d, nil
	case "f_trace":
		return None, nil
	case "f_trace_lines":
		return True, nil
	case "f_trace_opcodes":
		return False, nil
	case "f_lasti":
		return NewInt(-1), nil
	}
	return nil, Raise(AttributeError, "'frame' object has no attribute '%s'", name)
}

// codeLoadAttr answers the code attributes a frame exposes. The four modeled
// fields are faithful; co_flags reads 0 and any other co_ attribute is not
// modeled yet.
func codeLoadAttr(c *codeObject, name string) (Object, error) {
	switch name {
	case "co_filename":
		return NewStr(c.filename), nil
	case "co_name":
		return NewStr(c.name), nil
	case "co_qualname":
		return NewStr(c.qualname), nil
	case "co_firstlineno":
		return NewInt(int64(c.firstline)), nil
	case "co_flags":
		return NewInt(0), nil
	}
	return nil, Raise(AttributeError, "'code' object has no attribute '%s'", name)
}

// frameRepr and codeRepr mirror CPython's shape without the address, which is
// not reproducible; the fields that identify the frame are what a program keys
// on. The line is the live line for a frame and the first line for code.
func frameRepr(f *frameObject) string {
	return fmt.Sprintf("<frame at 0x0, file %q, line %d, code %s>", f.code.filename, f.line, f.code.name)
}

func codeRepr(c *codeObject) string {
	return fmt.Sprintf("<code object %s, file %q, line %d>", c.name, c.filename, c.firstline)
}
