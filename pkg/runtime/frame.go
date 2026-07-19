package runtime

import "github.com/tamnd/unagi/pkg/objects"

// unagi compiles Python to Go and keeps no interpreter frames, so sys._getframe
// would have nothing to walk. To give it a stack, every compiled Python function
// pushes one lightweight frame on entry and pops it on exit through a deferred
// call, and the module body pushes the bottom frame. The frame carries only what
// the stdlib reads through sys._getframe (f_back, f_code, f_locals type), which
// is enough for _collections_abc to take type(sys._getframe().f_locals); the
// per-line f_lineno and a faithful f_locals mapping are later slices.
//
// The stack lives on the *objects.Thread the call spine already threads through
// every frame, so a second thread walks its own frames and the push/pop needs no
// synchronization. PushFrame links f_back to the running top itself, so the
// emitted code passes nil for back and never names the unexported frame type.

// PushFrame builds a frame for the entering function and makes it the running
// frame on t. file/name/qual seed the code object and firstline is the def line,
// the value co_firstlineno reads back. optimized marks a function frame apart
// from the module body, deciding whether f_locals reads back as a proxy or a
// plain dict.
func PushFrame(t *objects.Thread, file, name, qual string, firstline int, optimized bool) {
	t.PushFrame(objects.NewFrame(nil, nil, file, name, qual, firstline, optimized))
}

// PopFrame drops the running frame as its function returns or unwinds, paired
// with PushFrame through a deferred call.
func PopFrame(t *objects.Thread) {
	t.PopFrame()
}
