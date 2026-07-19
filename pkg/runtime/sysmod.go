package runtime

import (
	"math"
	"runtime"

	"github.com/tamnd/unagi/pkg/objects"
)

// sysPlatform reports sys.platform for the host the compiled program runs on,
// mapping Go's GOOS to the value CPython uses: darwin and linux pass through,
// and windows reads as win32, the string the stdlib branches on.
func sysPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}

// sys is the first built-in module: the runtime registers it in the import
// table itself, so `import sys` works in every compiled program without a
// table entry from the build. sys.modules is the live dict the import
// machinery reads and writes, not a copy, which is what makes pokes, deletes,
// None entries, and sys.modules[__name__] = obj self-replacement take effect.
//
// The identity attributes below carry the pinned CPython's own values, so a
// floor module that gates on sys.version_info or reads sys.maxsize sees what it
// would under CPython 3.14.6. version_info is a plain tuple here rather than the
// struct sequence CPython uses: the >= and indexing a version gate needs behave
// identically, but the tm-style named fields (.major) and the
// sys.version_info(...) repr are an accepted divergence until the struct
// sequence type lands.

// The pinned oracle, mirrored from conformance/ORACLE_PIN. Moving the pin moves
// these in the same diff, so a version bump stays visible in review.
const (
	pyMajor        = 3
	pyMinor        = 14
	pyMicro        = 6
	pyReleaseLevel = "final"
	pySerial       = 0
)

func init() {
	moduleTable["sys"] = &moduleEntry{builtin: true, exec: initSys}
}

func initSys(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("modules", modules); err != nil {
		return err
	}
	versionInfo := objects.NewTuple([]objects.Object{
		objects.NewInt(pyMajor),
		objects.NewInt(pyMinor),
		objects.NewInt(pyMicro),
		objects.NewStr(pyReleaseLevel),
		objects.NewInt(pySerial),
	})
	// hexversion packs the version the way CPython's PY_VERSION_HEX macro does:
	// major, minor, micro in a byte each, then the release level nibble (final is
	// 0xF) and the serial nibble.
	hex := int64(pyMajor)<<24 | int64(pyMinor)<<16 | int64(pyMicro)<<8 | 0xF0 | int64(pySerial)
	attrs := []struct {
		name string
		val  objects.Object
	}{
		{"version_info", versionInfo},
		{"hexversion", objects.NewInt(hex)},
		{"maxsize", objects.NewInt(math.MaxInt64)},
		{"maxunicode", objects.NewInt(0x10FFFF)},
		{"byteorder", objects.NewStr("little")},
		{"platform", objects.NewStr(sysPlatform())},
	}
	for _, a := range attrs {
		if err := set(a.name, a.val); err != nil {
			return err
		}
	}
	return nil
}
