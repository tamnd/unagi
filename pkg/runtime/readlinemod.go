package runtime

import (
	"os"
	"strings"
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// readlineOSError maps a Go file error onto the Python exception readline's
// history-file calls raise: a missing file is FileNotFoundError, anything else
// is a plain OSError.
func readlineOSError(err error) error {
	if os.IsNotExist(err) {
		return objects.Raise("FileNotFoundError", "%s", err.Error())
	}
	return objects.Raise("OSError", "%s", err.Error())
}

// readline is the GNU readline / editline binding CPython uses for interactive
// line editing, history and completion. Without it `import readline` (and
// anything that optionally enables history, e.g. `cmd`, `code`, rlcompleter
// setups) raised ModuleNotFoundError.
//
// A compiled unagi program is not the interactive REPL readline drives, so the
// line-editing surface (the actual editing, key bindings, redisplay) has nothing
// to act on and degrades to a faithful no-op, which is exactly readline's
// observable behaviour when stdin is not a terminal. The parts a program's
// result can depend on are real: the history list (add/get/remove/replace/
// clear/length) and its file round-trip, the completer and the completer
// delimiters, and the stored hooks. This is the honest split, the same shape as
// the faulthandler and tracemalloc shims.
//
// The module is portable, so it registers on every target.

type readlineState struct {
	mu       sync.Mutex
	history  []string
	maxLen   int // set_history_length; -1 means unlimited
	completer objects.Object
	delims   string
	// Stored hooks, returned as set but never invoked (no interactive loop).
	startupHook  objects.Object
	preInputHook objects.Object
	displayHook  objects.Object
	autoHistory  bool
}

var readlineMod = &readlineState{
	maxLen:      -1,
	completer:   objects.None,
	delims:      " \t\n`~!@#$%^&*()-=+[{]}\\|;:'\",<>/?",
	startupHook: objects.None,
	preInputHook: objects.None,
	displayHook: objects.None,
	autoHistory: true,
}

func init() {
	moduleTable["readline"] = &moduleEntry{builtin: true, exec: initReadline}
}

func initReadline(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }
	fn := func(name string, arity int, f func([]objects.Object) (objects.Object, error)) error {
		return set(name, objects.NewFunc(name, arity, f))
	}

	// Identity attributes. backend reports which line-editing library is in use;
	// "readline" is the GNU default a program branches on.
	if err := set("backend", objects.NewStr("readline")); err != nil {
		return err
	}
	for _, name := range []string{"_READLINE_VERSION", "_READLINE_RUNTIME_VERSION"} {
		if err := set(name, objects.NewInt(0x0802)); err != nil {
			return err
		}
	}
	if err := set("_READLINE_LIBRARY_VERSION", objects.NewStr("8.2")); err != nil {
		return err
	}

	// History: the faithful half.
	if err := fn("add_history", 1, readlineAddHistory); err != nil {
		return err
	}
	if err := fn("clear_history", 0, readlineClearHistory); err != nil {
		return err
	}
	if err := fn("get_current_history_length", 0, readlineCurrentLen); err != nil {
		return err
	}
	if err := fn("get_history_length", 0, readlineGetHistoryLength); err != nil {
		return err
	}
	if err := fn("set_history_length", 1, readlineSetHistoryLength); err != nil {
		return err
	}
	if err := fn("get_history_item", 1, readlineGetHistoryItem); err != nil {
		return err
	}
	if err := fn("remove_history_item", 1, readlineRemoveHistoryItem); err != nil {
		return err
	}
	if err := fn("replace_history_item", 2, readlineReplaceHistoryItem); err != nil {
		return err
	}
	if err := fn("read_history_file", -1, readlineReadHistoryFile); err != nil {
		return err
	}
	if err := fn("write_history_file", -1, readlineWriteHistoryFile); err != nil {
		return err
	}
	if err := fn("append_history_file", -1, readlineAppendHistoryFile); err != nil {
		return err
	}

	// Completion: store and return, real state.
	if err := fn("set_completer", -1, readlineSetCompleter); err != nil {
		return err
	}
	if err := fn("get_completer", 0, readlineGetCompleter); err != nil {
		return err
	}
	if err := fn("set_completer_delims", 1, readlineSetDelims); err != nil {
		return err
	}
	if err := fn("get_completer_delims", 0, readlineGetDelims); err != nil {
		return err
	}
	if err := fn("get_completion_type", 0, func([]objects.Object) (objects.Object, error) { return objects.NewInt(9), nil }); err != nil {
		return err
	}
	if err := fn("get_begidx", 0, func([]objects.Object) (objects.Object, error) { return objects.NewInt(0), nil }); err != nil {
		return err
	}
	if err := fn("get_endidx", 0, func([]objects.Object) (objects.Object, error) { return objects.NewInt(0), nil }); err != nil {
		return err
	}

	// Line buffer: nothing is being edited, so it is empty.
	if err := fn("get_line_buffer", 0, func([]objects.Object) (objects.Object, error) { return objects.NewStr(""), nil }); err != nil {
		return err
	}

	// Hooks: stored as set, never fired (no interactive loop to fire them).
	if err := fn("set_startup_hook", -1, readlineSetStartupHook); err != nil {
		return err
	}
	if err := fn("set_pre_input_hook", -1, readlineSetPreInputHook); err != nil {
		return err
	}
	if err := fn("set_completion_display_matches_hook", -1, readlineSetDisplayHook); err != nil {
		return err
	}
	if err := fn("set_auto_history", 1, readlineSetAutoHistory); err != nil {
		return err
	}

	// Editing / display / init: no terminal to act on, faithful no-ops.
	for _, name := range []string{"insert_text", "redisplay", "parse_and_bind", "read_init_file"} {
		if err := fn(name, -1, func([]objects.Object) (objects.Object, error) { return objects.None, nil }); err != nil {
			return err
		}
	}
	return nil
}

func readlineAddHistory(args []objects.Object) (objects.Object, error) {
	s, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "add_history() argument must be str, not %s", args[0].TypeName())
	}
	readlineMod.mu.Lock()
	readlineMod.history = append(readlineMod.history, s)
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineClearHistory(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	readlineMod.history = nil
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineCurrentLen(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	n := len(readlineMod.history)
	readlineMod.mu.Unlock()
	return objects.NewInt(int64(n)), nil
}

func readlineGetHistoryLength(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	n := readlineMod.maxLen
	readlineMod.mu.Unlock()
	return objects.NewInt(int64(n)), nil
}

func readlineSetHistoryLength(args []objects.Object) (objects.Object, error) {
	n, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "set_history_length() argument must be int")
	}
	readlineMod.mu.Lock()
	readlineMod.maxLen = int(n)
	readlineMod.mu.Unlock()
	return objects.None, nil
}

// readlineGetHistoryItem is 1-based, like GNU readline; an out-of-range index
// returns None.
func readlineGetHistoryItem(args []objects.Object) (objects.Object, error) {
	idx, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "get_history_item() argument must be int")
	}
	readlineMod.mu.Lock()
	defer readlineMod.mu.Unlock()
	i := int(idx)
	if i < 1 || i > len(readlineMod.history) {
		return objects.None, nil
	}
	return objects.NewStr(readlineMod.history[i-1]), nil
}

func readlineRemoveHistoryItem(args []objects.Object) (objects.Object, error) {
	idx, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "remove_history_item() argument must be int")
	}
	readlineMod.mu.Lock()
	defer readlineMod.mu.Unlock()
	i := int(idx)
	if i < 0 || i >= len(readlineMod.history) {
		return nil, objects.Raise(objects.ValueError, "No history item at position %d", i)
	}
	readlineMod.history = append(readlineMod.history[:i], readlineMod.history[i+1:]...)
	return objects.None, nil
}

func readlineReplaceHistoryItem(args []objects.Object) (objects.Object, error) {
	idx, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "replace_history_item() argument 1 must be int")
	}
	s, ok := objects.AsStr(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "replace_history_item() argument 2 must be str")
	}
	readlineMod.mu.Lock()
	defer readlineMod.mu.Unlock()
	i := int(idx)
	if i < 0 || i >= len(readlineMod.history) {
		return nil, objects.Raise(objects.ValueError, "No history item at position %d", i)
	}
	readlineMod.history[i] = s
	return objects.None, nil
}

// readlineFilename resolves the optional filename argument. A missing or None
// filename means the default history file; since a compiled program has no
// interactive default, that case is a no-op (reported by ok=false).
func readlineFilename(args []objects.Object) (string, bool, error) {
	if len(args) == 0 || args[0] == objects.None {
		return "", false, nil
	}
	s, ok := objects.AsStr(args[0])
	if !ok {
		return "", false, objects.Raise(objects.TypeError, "argument must be str or None")
	}
	return s, true, nil
}

func readlineReadHistoryFile(args []objects.Object) (objects.Object, error) {
	name, ok, err := readlineFilename(args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return objects.None, nil
	}
	data, rerr := os.ReadFile(name)
	if rerr != nil {
		return nil, readlineOSError(rerr)
	}
	lines := strings.Split(string(data), "\n")
	readlineMod.mu.Lock()
	for _, ln := range lines {
		if ln != "" {
			readlineMod.history = append(readlineMod.history, ln)
		}
	}
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineWriteHistoryFile(args []objects.Object) (objects.Object, error) {
	name, ok, err := readlineFilename(args)
	if err != nil {
		return nil, err
	}
	if !ok {
		return objects.None, nil
	}
	if werr := os.WriteFile(name, []byte(readlineHistoryText()), 0o600); werr != nil {
		return nil, readlineOSError(werr)
	}
	return objects.None, nil
}

func readlineAppendHistoryFile(args []objects.Object) (objects.Object, error) {
	// append_history_file(nelements, filename=None): append the last nelements
	// history entries. Without a filename it is a no-op like the others.
	var name string
	var ok bool
	var err error
	if len(args) >= 2 {
		name, ok, err = readlineFilename(args[1:])
	}
	if err != nil {
		return nil, err
	}
	if !ok {
		return objects.None, nil
	}
	n := 0
	if len(args) >= 1 {
		if v, isInt := objects.AsInt(args[0]); isInt {
			n = int(v)
		}
	}
	readlineMod.mu.Lock()
	hist := readlineMod.history
	if n > 0 && n < len(hist) {
		hist = hist[len(hist)-n:]
	}
	text := strings.Join(hist, "\n")
	readlineMod.mu.Unlock()
	if text != "" {
		text += "\n"
	}
	f, oerr := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if oerr != nil {
		return nil, readlineOSError(oerr)
	}
	defer f.Close()
	if _, werr := f.WriteString(text); werr != nil {
		return nil, readlineOSError(werr)
	}
	return objects.None, nil
}

// readlineHistoryText renders the history for write_history_file, honouring a
// set_history_length cap the way GNU readline does.
func readlineHistoryText() string {
	readlineMod.mu.Lock()
	defer readlineMod.mu.Unlock()
	hist := readlineMod.history
	if readlineMod.maxLen >= 0 && readlineMod.maxLen < len(hist) {
		hist = hist[len(hist)-readlineMod.maxLen:]
	}
	if len(hist) == 0 {
		return ""
	}
	return strings.Join(hist, "\n") + "\n"
}

func readlineSetCompleter(args []objects.Object) (objects.Object, error) {
	c := objects.Object(objects.None)
	if len(args) >= 1 {
		c = args[0]
	}
	readlineMod.mu.Lock()
	readlineMod.completer = c
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineGetCompleter(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	c := readlineMod.completer
	readlineMod.mu.Unlock()
	return c, nil
}

func readlineSetDelims(args []objects.Object) (objects.Object, error) {
	s, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "set_completer_delims() argument must be str")
	}
	readlineMod.mu.Lock()
	readlineMod.delims = s
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineGetDelims(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	d := readlineMod.delims
	readlineMod.mu.Unlock()
	return objects.NewStr(d), nil
}

func readlineSetStartupHook(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	if len(args) >= 1 {
		readlineMod.startupHook = args[0]
	} else {
		readlineMod.startupHook = objects.None
	}
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineSetPreInputHook(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	if len(args) >= 1 {
		readlineMod.preInputHook = args[0]
	} else {
		readlineMod.preInputHook = objects.None
	}
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineSetDisplayHook(args []objects.Object) (objects.Object, error) {
	readlineMod.mu.Lock()
	if len(args) >= 1 {
		readlineMod.displayHook = args[0]
	} else {
		readlineMod.displayHook = objects.None
	}
	readlineMod.mu.Unlock()
	return objects.None, nil
}

func readlineSetAutoHistory(args []objects.Object) (objects.Object, error) {
	t, err := objects.TruthOf(args[0])
	if err != nil {
		return nil, err
	}
	readlineMod.mu.Lock()
	readlineMod.autoHistory = t
	readlineMod.mu.Unlock()
	return objects.None, nil
}
