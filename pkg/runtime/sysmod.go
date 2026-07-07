package runtime

import "github.com/tamnd/unagi/pkg/objects"

// sys is the first built-in module: the runtime registers it in the import
// table itself, so `import sys` works in every compiled program without a
// table entry from the build. Only the registry surface ships so far;
// sys.modules is the live dict the import machinery reads and writes, not a
// copy, which is what makes pokes, deletes, None entries, and
// sys.modules[__name__] = obj self-replacement take effect.

func init() {
	moduleTable["sys"] = &moduleEntry{builtin: true, exec: initSys}
}

func initSys(m *objects.Module) error {
	return objects.StoreAttr(m, "modules", modules)
}
