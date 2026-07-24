# _imp is the builtin component importlib is written on top of. It carries the
# bytecode magic token, the extension-suffix list, the import lock primitives,
# and the builtin and frozen probes _bootstrap consults. unagi runs its imports
# through its own machinery rather than through importlib, so this exercises the
# surface directly rather than driving a real import through it.
import _imp

print("magic", _imp.pyc_magic_number_token)
print("chbp", _imp.check_hash_based_pycs)

# Nothing is frozen in unagi, so is_frozen is always false.
print("is_frozen sys", _imp.is_frozen("sys"))
print("is_frozen nope", _imp.is_frozen("no_such_module_xyz"))

# sys is a builtin module everywhere; a made-up name is not. is_builtin returns
# an int the way the C primitive does, so bool() normalizes it for the compare.
print("is_builtin sys", int(bool(_imp.is_builtin("sys"))))
print("is_builtin missing", int(bool(_imp.is_builtin("no_such_module_xyz"))))

# extension_suffixes lists the suffixes a dynamic extension could take. The exact
# set is platform specific, so only the shape and the bare .so entry are checked.
suffixes = _imp.extension_suffixes()
print("suffixes is list", isinstance(suffixes, list))
print("suffixes has .so", ".so" in suffixes)

# The import lock is a no-op here: acquire and release return None and the lock
# never reads as held.
print("acquire", _imp.acquire_lock())
print("release", _imp.release_lock())
print("lock_held", _imp.lock_held())

# source_hash stamps a hash-based pyc; unagi never writes one, but the primitive
# still returns eight bytes of the right shape.
h = _imp.source_hash(_imp.pyc_magic_number_token, b"abc")
print("source_hash is bytes", isinstance(h, bytes))
print("source_hash len", len(h))

# create_builtin materializes a real native module by name, the bridge
# _bootstrap._setup uses to pull in _thread; the result is that module object.
class Spec:
    name = "_thread"
mod = _imp.create_builtin(Spec())
print("create_builtin _thread", mod.__name__)
print("has allocate_lock", hasattr(mod, "allocate_lock"))
