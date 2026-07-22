# The vendored CPython os.py / posixpath.py / genericpath.py now provide `os`
# and `os.path` on top of the posix accelerator, so the two hand-written Go
# shims are gone. This exercises the common surface: module identity, path
# algebra, the environ mapping, and the stat/scandir calls the floor routes
# through posix. Everything printed is host invariant, so the darwin oracle and
# the Linux corpus agree.
import os
import os.path
import stat

print("name", os.name)
print("sep", os.sep)
print("pathsep", os.pathsep)
print("path_is_posixpath", os.path.__name__ == "posixpath")

print("cwd_is_str", isinstance(os.getcwd(), str))
print("join", os.path.join("a", "b", "c"))
print("join_abs", os.path.join("a", "/b", "c"))
print("split", os.path.split("/a/b/c.txt"))
print("splitext", os.path.splitext("foo.tar.gz"))
print("basename", os.path.basename("/a/b/c.txt"))
print("dirname", os.path.dirname("/a/b/c.txt"))
print("normpath", os.path.normpath("a/./b/../c"))
print("isabs_true", os.path.isabs("/a"))
print("isabs_false", os.path.isabs("a"))
print("abspath_is_str", isinstance(os.path.abspath("x"), str))

print("environ_type", type(os.environ).__name__)
print("getenv_missing", os.getenv("UNAGI_NO_SUCH_VAR_XYZ") is None)
print("getenv_default", os.getenv("UNAGI_NO_SUCH_VAR_XYZ", "d"))

print("fspath", os.fspath("q"))
print("fsencode", os.fsencode("q"))
print("fsdecode", os.fsdecode(b"q"))

# stat and scandir go through the posix accelerator. Assert structural
# properties only, never a concrete mode or size that differs across hosts.
st = os.stat(".")
print("stat_isdir", stat.S_ISDIR(st.st_mode))
print("getsize_int", isinstance(os.path.getsize("."), int))
print("exists_dot", os.path.exists("."))
print("isdir_dot", os.path.isdir("."))
print("isfile_dot", os.path.isfile("."))
print("listdir_is_list", isinstance(os.listdir("."), list))
print("scandir_names_list", isinstance(sorted(e.name for e in os.scandir(".")), list))
print("getpid_gt0", os.getpid() > 0)
