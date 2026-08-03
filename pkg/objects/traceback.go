package objects

import (
	"fmt"
	"reflect"
)

// tracebackObject is the first-class traceback exposed as exc.__traceback__ and
// sys.exc_info()[2]. unagi compiles to Go and keeps no interpreter frames, so a
// raised exception records a lightweight []Frame (innermost raise site first);
// this type projects that list into the tb_frame/tb_lineno/tb_next chain the
// traceback module walks. The chain is ordered outermost frame first, with
// tb_next stepping inward toward the raise site, matching CPython's tb layout so
// traceback.extract_tb, traceback.format_exception and a manual tb_next walk all
// read back the same frames the raw uncaught renderer prints.
type tracebackObject struct {
	frame  *frameObject
	lineno int
	next   *tracebackObject
}

func (*tracebackObject) TypeName() string { return "traceback" }

// tracebackFromFrames builds the tb chain from an exception's recorded frames.
// e.Frames collects innermost first, while a CPython traceback heads at the
// outermost frame and tb_next walks inward, so the chain is assembled by
// stepping the slice from its tail (outermost) down to the raise site. Each node
// carries a thin frame whose code object names the file and function and whose
// f_lineno matches tb_lineno, which is all the traceback module reads; f_globals
// is left empty so linecache falls back to reading the source file directly.
func tracebackFromFrames(frames []Frame) *tracebackObject {
	var head *tracebackObject
	for _, fr := range frames {
		fo := NewFrame(nil, nil, fr.File, fr.Func, fr.Func, fr.Line, false)
		fo.SetLine(fr.Line)
		head = &tracebackObject{frame: fo, lineno: fr.Line, next: head}
	}
	return head
}

// tracebackLoadAttr answers the four attributes the traceback module and a
// hand-written walk read: tb_frame, tb_lineno, tb_next and tb_lasti. tb_lasti is
// the last executed bytecode offset, which the compiled model has no equivalent
// for, so it reads back -1, the value CPython also uses for a frame that has not
// run an instruction.
func tracebackLoadAttr(tb *tracebackObject, name string) (Object, error) {
	switch name {
	case "tb_frame":
		return tb.frame, nil
	case "tb_lineno":
		return NewInt(int64(tb.lineno)), nil
	case "tb_lasti":
		return NewInt(-1), nil
	case "tb_next":
		if tb.next == nil {
			return None, nil
		}
		return tb.next, nil
	}
	return nil, Raise(AttributeError, "'traceback' object has no attribute '%s'", name)
}

// tracebackRepr mirrors CPython's <traceback object at 0x...>; the address is
// scrubbed by the conformance harness, so a real pointer keeps the shape without
// pinning a value.
func tracebackRepr(tb *tracebackObject) string {
	return fmt.Sprintf("<traceback object at 0x%012x>", reflect.ValueOf(tb).Pointer())
}
