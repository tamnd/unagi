package runtime

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// handledStack tracks the exception each active except block is handling,
// innermost last. The emitter calls PushHandled on entry to a handler and
// PopHandled on every exit path. A plain package-level slice is enough:
// emitted programs are single-threaded until M5 brings goroutines.
var handledStack []*objects.Exception

// IsExc reports whether err is a Python-level exception.
func IsExc(err error) bool {
	_, ok := err.(*objects.Exception)
	return ok
}

// ExcObj returns the exception object behind err for `except ... as e`
// binding, or nil when err is not a Python exception.
func ExcObj(err error) objects.Object {
	if e, ok := err.(*objects.Exception); ok {
		return e
	}
	return nil
}

// ExcMatches reports whether err is an exception matching any of the
// given classes. Non-exception errors match nothing.
func ExcMatches(err error, classes ...string) bool {
	e, ok := err.(*objects.Exception)
	if !ok {
		return false
	}
	for _, c := range classes {
		if objects.Matches(e.Kind, c) {
			return true
		}
	}
	return false
}

// PushHandled records err as the exception now being handled. A non
// exception err pushes a nil slot so PopHandled stays balanced.
func PushHandled(err error) {
	e, _ := err.(*objects.Exception)
	handledStack = append(handledStack, e)
}

// PopHandled drops the innermost handled exception.
func PopHandled() {
	if n := len(handledStack); n > 0 {
		handledStack = handledStack[:n-1]
	}
}

// ExcStarSplit partitions err by whether its leaves match any of the given
// classes, for one except* clause. It returns the matched part and the
// unhandled remainder, either of which is a nil error when empty. A non
// exception err matches nothing and stays whole in rest.
func ExcStarSplit(err error, classes ...string) (matched, rest error) {
	e, ok := err.(*objects.Exception)
	if !ok {
		return nil, err
	}
	m, r := objects.SplitStar(e, classes)
	if m != nil {
		matched = m
	}
	if r != nil {
		rest = r
	}
	return matched, rest
}

// ExcStarCombine folds the exceptions raised by the except* handlers back
// together with the unhandled remainder into the one exception that leaves
// the try, or nil when nothing is left to propagate. Nil entries in raised
// are dropped so an empty accumulator combines to just the remainder.
func ExcStarCombine(rest error, raised []error) error {
	re, _ := rest.(*objects.Exception)
	var rs []*objects.Exception
	for _, r := range raised {
		if e, ok := r.(*objects.Exception); ok {
			rs = append(rs, e)
		}
	}
	if c := objects.CombineStar(re, rs); c != nil {
		return c
	}
	return nil
}

// NewExc constructs an exception object without raising it, the
// ExceptionClass(args...) expression.
func NewExc(class string, args []objects.Object) objects.Object {
	return objects.NewException(class, args)
}

// NewExcGroup constructs an ExceptionGroup or BaseExceptionGroup, the
// one exception constructor that can itself raise: the message and the
// sub-exception sequence validate eagerly.
func NewExcGroup(class string, args []objects.Object) (objects.Object, error) {
	return objects.NewExcGroup(class, args)
}

// chainInto sets newer.Context = pending following CPython's
// PyErr_SetObject rules: never link an exception to itself, and break
// any link back to newer inside pending's context chain first so no
// cycle forms. Probed on 3.14: re-raising the handled exception leaves
// its context alone, and re-raising an outer pending exception inside a
// nested handler unlinks the inner exception's back-reference.
func chainInto(newer, pending *objects.Exception) {
	if newer == nil || pending == nil || newer == pending {
		return
	}
	for o := pending; o.Context != nil; o = o.Context {
		if o.Context == newer {
			o.Context = nil
			break
		}
	}
	newer.Context = pending
}

// RaiseObj raises an exception object: `raise e`. Non-exception values
// get CPython's TypeError. The implicit context comes from the top of
// the handled stack.
func RaiseObj(o objects.Object) error {
	e, ok := o.(*objects.Exception)
	if !ok {
		// Probed on 3.14: raise 42.
		return objects.Raise(objects.TypeError, "exceptions must derive from BaseException")
	}
	// An explicit `raise e` unwinds normally, so every frame including
	// the raise line lands in the traceback. Only a bare raise skips.
	e.Reraised = false
	if len(handledStack) > 0 {
		chainInto(e, handledStack[len(handledStack)-1])
	}
	return e
}

// RaiseBare re-raises the exception being handled: bare `raise`.
// Probed on 3.14: with nothing active the message is exactly
// "No active exception to reraise" (no hyphen), and the re-raised
// traceback keeps the original raise-site line for the re-raising
// function without adding an entry for the bare raise itself, so the
// Reraised flag tells TB to skip exactly one frame.
func RaiseBare() error {
	if n := len(handledStack); n > 0 {
		if e := handledStack[n-1]; e != nil {
			e.Reraised = true
			return e
		}
	}
	return objects.Raise(objects.RuntimeError, "No active exception to reraise")
}

// SetCause implements `raise X from Y`. from None clears the cause and
// suppresses the context; an exception cause is recorded and also
// suppresses the context; anything else is CPython's TypeError.
func SetCause(err error, cause objects.Object, fromNone bool) error {
	e, ok := err.(*objects.Exception)
	if !ok {
		return err
	}
	if fromNone {
		e.Cause = nil
		e.SuppressContext = true
		return e
	}
	c, ok := cause.(*objects.Exception)
	if !ok {
		// Probed on 3.14: raise ValueError("x") from 42.
		return objects.Raise(objects.TypeError, "exception causes must derive from BaseException")
	}
	e.Cause = c
	e.SuppressContext = true
	return e
}

// WithExit runs a context manager's __exit__ on the way out of a with block.
// exc is the exception leaving the body, or nil on a normal exit or a parked
// return, break, or continue. On a non-exception exit __exit__ receives
// (None, None, None) and its return value is ignored; on the exception path it
// receives the exception's type and value, a None traceback, and a truthy
// return suppresses the exception. A __exit__ that itself raises replaces exc,
// which chains in as its context. The third argument is None because unagi has
// no first-class traceback objects yet.
//
// The second result reports whether __exit__ itself raised. Only then does the
// error leave through the with statement's own frame, so the caller stamps a
// traceback entry in that one case; a body exception that __exit__ merely lets
// through already carries its frame from the raise site.
func WithExit(exitFn objects.Object, exc error) (error, bool) {
	et, ev := objects.None, objects.None
	pe, _ := exc.(*objects.Exception)
	if pe != nil {
		et = objects.ExcType(pe.Kind)
		ev = pe
	}
	res, cerr := objects.Call(exitFn, []objects.Object{et, ev, objects.None})
	if cerr != nil {
		return ChainContext(cerr, exc), true
	}
	if pe == nil {
		return nil, false
	}
	if objects.Truth(res) {
		return nil, false
	}
	return exc, false
}

// ChainContext links a pending exception under a newer one, the case
// where a finally block raises over an in-flight exception. Same
// self-reference rules as the implicit raise chaining.
func ChainContext(newer, pending error) error {
	ne, ok1 := newer.(*objects.Exception)
	pe, ok2 := pending.(*objects.Exception)
	if ok1 && ok2 {
		chainInto(ne, pe)
	}
	return newer
}

// TB appends one traceback frame as err unwinds through a Python frame.
// The emitter guarantees exactly one call per frame per unwind. A bare
// re-raise consumes its Reraised flag here instead of appending, which
// reproduces the 3.14 rendering where the re-raising function shows its
// original raise-site line only.
func TB(err error, file string, line int, fn string) error {
	e, ok := err.(*objects.Exception)
	if !ok {
		return err
	}
	if e.Reraised {
		e.Reraised = false
		return e
	}
	e.Frames = append(e.Frames, objects.Frame{File: file, Line: line, Func: fn})
	return e
}

// srcLines is the compiled program's Python source split into lines,
// registered by the generated main so frame lines can quote their
// source. M1 compiles a single file, so one slice is enough. Nil means
// no source was embedded and frames render bare, which matches what
// CPython prints when the source file is gone.
var srcLines []string

// SetSource registers the embedded Python source for excerpt rendering.
func SetSource(src string) {
	srcLines = strings.Split(src, "\n")
}

// srcLine returns the stripped text of a 1-based source line, or ""
// when the line is out of range or blank. Blank excerpts print nothing,
// CPython's `if line:` guard in traceback.py.
func srcLine(n int) string {
	if n < 1 || n > len(srcLines) {
		return ""
	}
	return strings.TrimSpace(srcLines[n-1])
}

// PrintUncaught writes the CPython-3.14-shaped report for an uncaught
// exception to Stderr. Causes and contexts render first, depth first,
// with CPython's connective lines. Frame lines carry a source excerpt
// when the binary embeds its source; caret and anchor lines never
// appear because compiled code does not track columns.
func PrintUncaught(err error) {
	var b strings.Builder
	if e, ok := err.(*objects.Exception); ok {
		renderChain(&b, &excPrintCtx{}, buildTree(e))
	} else {
		// Not a Python exception; keep the old minimal shape.
		fmt.Fprintf(&b, "Traceback (most recent call last):\n%s\n", err.Error())
	}
	// Nothing sensible to do if stderr itself is gone.
	_, _ = io.WriteString(Stderr, b.String())
}

// Exception group rendering caps, CPython's max_group_width and
// max_group_depth in traceback.py.
const (
	maxGroupWidth = 15
	maxGroupDepth = 10
)

// excNode is one exception in the report tree. The tree fixes, ahead of
// rendering, where each cause and context prints: CPython builds its
// TracebackException tree with a LIFO queue and a seen set keyed by
// identity, so when the same exception is reachable twice (say a group
// child that is also the group's context), whichever queue entry pops
// first expands the chain and the other renders bare. Group children
// pop before the group's own context, which is why a shared chain shows
// up inside the box and not above it.
type excNode struct {
	e        *objects.Exception
	cause    *excNode
	context  *excNode
	children []*excNode
}

// buildTree mirrors TracebackException.__init__: a LIFO queue walk
// where causes and contexts expand only on first sight but group
// children always get a node.
func buildTree(root *objects.Exception) *excNode {
	seen := map[*objects.Exception]bool{root: true}
	rootN := &excNode{e: root}
	queue := []*excNode{rootN}
	for len(queue) > 0 {
		n := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		e := n.e
		if e.Cause != nil && !seen[e.Cause] {
			seen[e.Cause] = true
			n.cause = &excNode{e: e.Cause}
		}
		// CPython's compact mode: the context only prints when no cause
		// slot was filled and the exception does not suppress it.
		if n.cause == nil && !e.SuppressContext && e.Context != nil && !seen[e.Context] {
			seen[e.Context] = true
			n.context = &excNode{e: e.Context}
		}
		for _, ch := range e.Group {
			seen[ch] = true
			n.children = append(n.children, &excNode{e: ch})
		}
		if n.cause != nil {
			queue = append(queue, n.cause)
		}
		if n.context != nil {
			queue = append(queue, n.context)
		}
		queue = append(queue, n.children...)
	}
	return rootN
}

// excPrintCtx carries the exception-group rendering state, CPython's
// _ExceptionPrintContext: the box depth and the shared need-close flag
// that lets a nested group's closing line stand in for its parent's.
type excPrintCtx struct {
	depth     int
	needClose bool
}

func (c *excPrintCtx) indent() string {
	return strings.Repeat("  ", c.depth)
}

// emit writes one report line. Inside a group box every line carries
// the two-space-per-level margin and the bar, blank lines keeping the
// bar's trailing space, which CPython emits too.
func (c *excPrintCtx) emit(b *strings.Builder, text string) {
	c.emitMargin(b, text, "|")
}

func (c *excPrintCtx) emitMargin(b *strings.Builder, text, margin string) {
	if c.depth > 0 {
		b.WriteString(c.indent())
		b.WriteString(margin + " ")
	}
	b.WriteString(text)
	b.WriteByte('\n')
}

// renderChain renders n preceded by its cause or context with the
// connective lines between them, all at the current box depth.
func renderChain(b *strings.Builder, c *excPrintCtx, n *excNode) {
	if n.cause != nil {
		renderChain(b, c, n.cause)
		c.emit(b, "")
		c.emit(b, "The above exception was the direct cause of the following exception:")
		c.emit(b, "")
	} else if n.context != nil {
		renderChain(b, c, n.context)
		c.emit(b, "")
		c.emit(b, "During handling of the above exception, another exception occurred:")
		c.emit(b, "")
	}
	if n.children != nil {
		renderGroup(b, c, n)
		return
	}
	if len(n.e.Frames) > 0 {
		c.emit(b, "Traceback (most recent call last):")
		renderFrames(b, c, n.e)
	}
	renderMessage(b, c, n.e)
}

func renderFrames(b *strings.Builder, c *excPrintCtx, e *objects.Exception) {
	for i := len(e.Frames) - 1; i >= 0; i-- {
		f := e.Frames[i]
		c.emit(b, fmt.Sprintf("  File %q, line %d, in %s", f.File, f.Line, f.Func))
		if l := srcLine(f.Line); l != "" {
			c.emit(b, "    "+l)
		}
	}
}

// renderMessage prints the final Kind: message line and the PEP 678
// notes, multi-line notes spanning as many report lines as they contain.
func renderMessage(b *strings.Builder, c *excPrintCtx, e *objects.Exception) {
	c.emit(b, e.Error())
	for _, n := range e.Notes {
		for _, l := range strings.Split(n, "\n") {
			c.emit(b, l)
		}
	}
}

// renderGroup draws the 3.14 exception group box: the group's own
// traceback and message inside a "| " margin, then each sub-exception
// between numbered dash separators, one margin deeper per nested group.
// Only a group at depth zero bumps the depth for its own lines and
// marks its header with "+". Width caps at maxGroupWidth sub-exceptions
// and depth at maxGroupDepth nested groups, with CPython's ellipsis
// lines for both. The closing dashes follow CPython's need-close flag:
// when the last child is itself a group, that child's closing line
// serves for both boxes.
func renderGroup(b *strings.Builder, c *excPrintCtx, n *excNode) {
	if c.depth > maxGroupDepth {
		c.emit(b, fmt.Sprintf("... (max_group_depth is %d)", maxGroupDepth))
		return
	}
	isTop := c.depth == 0
	if isTop {
		c.depth++
	}
	e := n.e
	if len(e.Frames) > 0 {
		header := "Exception Group Traceback (most recent call last):"
		if isTop {
			c.emitMargin(b, header, "+")
		} else {
			c.emit(b, header)
		}
		renderFrames(b, c, e)
	}
	renderMessage(b, c, e)

	total := len(n.children)
	slots := total
	if total > maxGroupWidth {
		slots = maxGroupWidth + 1
	}
	c.needClose = false
	for i := 0; i < slots; i++ {
		last := i == slots-1
		if last {
			c.needClose = true
		}
		truncated := i >= maxGroupWidth
		title := strconv.Itoa(i + 1)
		if truncated {
			title = "..."
		}
		marker := "  "
		if i == 0 {
			marker = "+-"
		}
		dashes := strings.Repeat("-", 16)
		b.WriteString(c.indent() + marker + "+" + dashes + " " + title + " " + dashes + "\n")
		c.depth++
		if truncated {
			rest := total - maxGroupWidth
			plural := "s"
			if rest == 1 {
				plural = ""
			}
			c.emit(b, fmt.Sprintf("and %d more exception%s", rest, plural))
		} else {
			renderChain(b, c, n.children[i])
		}
		if last && c.needClose {
			b.WriteString(c.indent() + "+" + strings.Repeat("-", 36) + "\n")
			c.needClose = false
		}
		c.depth--
	}
	if isTop {
		c.depth = 0
	}
}
