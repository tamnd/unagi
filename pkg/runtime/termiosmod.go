//go:build darwin || linux

package runtime

import (
	"syscall"
	"unsafe"

	"github.com/tamnd/unagi/pkg/objects"
)

// termios is the POSIX terminal-attribute accelerator. tty and pty drive a
// terminal into raw or cbreak mode through it: tcgetattr reads the current
// attributes as the [iflag, oflag, cflag, lflag, ispeed, ospeed, cc] list
// CPython uses, the caller edits the flag words and cc slots, and tcsetattr
// writes them back. The flag bits (ECHO, ICANON, ...) and the VMIN/VTIME cc
// indices come from Go's syscall constants so they carry the host's real values,
// the way CPython takes them from <termios.h>. The when-constants are fixed by
// POSIX and the struct packing is the only platform seam, kept in
// termios_darwin.go and termios_linux.go.
//
// tcgetattr returns cc the way CPython does: each control character is a
// length-one bytes object, except the VMIN and VTIME slots, which are integers
// because they hold counts rather than characters. tcsetattr accepts either form
// back, so a mode read, edited (tty sets cc[VMIN]=1) and written round-trips.

var termiosErrorClass objects.Object

// The tcsetattr "when" arguments. POSIX fixes these three at 0/1/2 on every host,
// so they are named here and mapped to the platform ioctl request in
// termiosWriteReq.
const (
	tcsaNow   = 0
	tcsaDrain = 1
	tcsaFlush = 2
)

func init() {
	moduleTable["termios"] = &moduleEntry{builtin: true, exec: initTermios}
}

// termiosConsts is the name->value table. The flag bits and the two cc indices
// come from syscall so they carry the host's real values; the three
// when-constants are the POSIX-fixed 0/1/2. The set is scoped to the surface tty
// and pty reach through; a host that needs a bit not listed here gets a clean
// AttributeError rather than a wrong value.
var termiosConsts = []struct {
	name string
	val  int
}{
	{"TCSANOW", tcsaNow},
	{"TCSADRAIN", tcsaDrain},
	{"TCSAFLUSH", tcsaFlush},
	{"VMIN", syscall.VMIN},
	{"VTIME", syscall.VTIME},
	// Input flags.
	{"IGNBRK", syscall.IGNBRK},
	{"BRKINT", syscall.BRKINT},
	{"IGNPAR", syscall.IGNPAR},
	{"PARMRK", syscall.PARMRK},
	{"INPCK", syscall.INPCK},
	{"ISTRIP", syscall.ISTRIP},
	{"INLCR", syscall.INLCR},
	{"IGNCR", syscall.IGNCR},
	{"ICRNL", syscall.ICRNL},
	{"IXON", syscall.IXON},
	{"IXANY", syscall.IXANY},
	{"IXOFF", syscall.IXOFF},
	// Output flags.
	{"OPOST", syscall.OPOST},
	// Control flags.
	{"PARENB", syscall.PARENB},
	{"CSIZE", syscall.CSIZE},
	{"CS8", syscall.CS8},
	// Local flags.
	{"ECHO", syscall.ECHO},
	{"ECHOE", syscall.ECHOE},
	{"ECHOK", syscall.ECHOK},
	{"ECHONL", syscall.ECHONL},
	{"ICANON", syscall.ICANON},
	{"IEXTEN", syscall.IEXTEN},
	{"ISIG", syscall.ISIG},
	{"NOFLSH", syscall.NOFLSH},
	{"TOSTOP", syscall.TOSTOP},
}

func initTermios(m *objects.Module) error {
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "termios: Exception base is unavailable")
	}
	errCls, err := objects.NewClass("error", "termios.error", []objects.Object{exc}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	termiosErrorClass = errCls
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("error", errCls); err != nil {
		return err
	}
	if err := set("tcgetattr", objects.NewFunc("tcgetattr", 1, termiosTcgetattr)); err != nil {
		return err
	}
	if err := set("tcsetattr", objects.NewFunc("tcsetattr", 3, termiosTcsetattr)); err != nil {
		return err
	}
	for _, c := range termiosConsts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	return nil
}

// termiosError raises a termios.error carrying the message, falling back to a
// plain OSError if the class is somehow unavailable.
func termiosError(msg string) error {
	if termiosErrorClass != nil {
		if inst, err := objects.Call(termiosErrorClass, []objects.Object{objects.NewStr(msg)}); err == nil {
			if e, ok := inst.(error); ok {
				return e
			}
		}
	}
	return objects.Raise("OSError", "%s", msg)
}

// termiosFd reads the file-descriptor argument the way CPython does: an int is
// taken directly, and any object with fileno() is asked for its descriptor.
func termiosFd(fn string, o objects.Object) (int, error) {
	if n, ok := objects.AsInt(o); ok {
		return int(n), nil
	}
	if v, err := objects.CallMethod(o, "fileno", nil); err == nil {
		if n, ok := objects.AsInt(v); ok {
			return int(n), nil
		}
	}
	return 0, objects.Raise(objects.TypeError, "%s() argument must be an int or have a fileno() method", fn)
}

// termiosGet runs the read-attributes ioctl into t.
func termiosGet(fd int, t *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), termiosReadReq, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

// termiosSet runs the write-attributes ioctl selected by when.
func termiosSet(fd, when int, t *syscall.Termios) error {
	req, ok := termiosWriteReq(when)
	if !ok {
		return objects.Raise(objects.TypeError, "tcsetattr: when must be TCSANOW, TCSADRAIN or TCSAFLUSH")
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

// termiosTcgetattr is termios.tcgetattr(fd): the terminal attributes as
// [iflag, oflag, cflag, lflag, ispeed, ospeed, cc].
func termiosTcgetattr(args []objects.Object) (objects.Object, error) {
	fd, err := termiosFd("tcgetattr", args[0])
	if err != nil {
		return nil, err
	}
	var t syscall.Termios
	if err := termiosGet(fd, &t); err != nil {
		return nil, termiosError(err.Error())
	}
	// The VMIN and VTIME slots share their array positions with the VEOF and
	// VEOL characters: they hold the non-canonical MIN/TIME counts only when
	// ICANON is clear, and hold the canonical end-of-file/line characters when it
	// is set. CPython returns the two slots as ints in the first case and as
	// bytes in the second, so mirror that; every other slot is always a
	// length-one bytes object.
	noncanon := t.Lflag&syscall.ICANON == 0
	cc := make([]objects.Object, len(t.Cc))
	for i := range t.Cc {
		if noncanon && (i == syscall.VMIN || i == syscall.VTIME) {
			cc[i] = objects.NewInt(int64(t.Cc[i]))
		} else {
			cc[i] = objects.NewBytes([]byte{byte(t.Cc[i])})
		}
	}
	return objects.NewList([]objects.Object{
		objects.NewInt(int64(t.Iflag)),
		objects.NewInt(int64(t.Oflag)),
		objects.NewInt(int64(t.Cflag)),
		objects.NewInt(int64(t.Lflag)),
		objects.NewInt(int64(t.Ispeed)),
		objects.NewInt(int64(t.Ospeed)),
		objects.NewList(cc),
	}), nil
}

// termiosTcsetattr is termios.tcsetattr(fd, when, attributes): write the
// [iflag, oflag, cflag, lflag, ispeed, ospeed, cc] list back to the terminal.
func termiosTcsetattr(args []objects.Object) (objects.Object, error) {
	fd, err := termiosFd("tcsetattr", args[0])
	if err != nil {
		return nil, err
	}
	when, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "tcsetattr: when must be an int")
	}
	n, err := objects.Len(args[2])
	if err != nil {
		return nil, err
	}
	if n != 7 {
		return nil, termiosError("tcsetattr, arg 3: must be 7 element list")
	}
	field := func(i int) (int64, error) {
		v, err := objects.GetItem(args[2], objects.NewInt(int64(i)))
		if err != nil {
			return 0, err
		}
		x, ok := objects.AsInt(v)
		if !ok {
			return 0, termiosError("tcsetattr, arg 3: bad attribute value")
		}
		return x, nil
	}
	var t syscall.Termios
	// Start from the current attributes so any field the struct carries that the
	// list does not model (the linux line discipline byte) keeps its value.
	if err := termiosGet(fd, &t); err != nil {
		return nil, termiosError(err.Error())
	}
	vals := make([]int64, 6)
	for i := 0; i < 6; i++ {
		if vals[i], err = field(i); err != nil {
			return nil, err
		}
	}
	t.Iflag = tcflag(vals[0])
	t.Oflag = tcflag(vals[1])
	t.Cflag = tcflag(vals[2])
	t.Lflag = tcflag(vals[3])
	t.Ispeed = speedval(vals[4])
	t.Ospeed = speedval(vals[5])
	ccList, err := objects.GetItem(args[2], objects.NewInt(6))
	if err != nil {
		return nil, err
	}
	ccn, err := objects.Len(ccList)
	if err != nil {
		return nil, err
	}
	if ccn != len(t.Cc) {
		return nil, termiosError("tcsetattr, arg 3: bad cc list length")
	}
	for i := 0; i < ccn; i++ {
		v, err := objects.GetItem(ccList, objects.NewInt(int64(i)))
		if err != nil {
			return nil, err
		}
		// A cc slot is either a length-one bytes object (a control character) or
		// an int (the VMIN/VTIME counts); accept both, matching CPython.
		if b, ok := objects.AsBytes(v); ok {
			if len(b) != 1 {
				return nil, termiosError("tcsetattr, arg 3: control character must be one byte")
			}
			t.Cc[i] = ccval(b[0])
			continue
		}
		if x, ok := objects.AsInt(v); ok {
			t.Cc[i] = ccval(x)
			continue
		}
		return nil, termiosError("tcsetattr, arg 3: bad cc value")
	}
	if err := termiosSet(fd, int(when), &t); err != nil {
		return nil, termiosError(err.Error())
	}
	return objects.None, nil
}
