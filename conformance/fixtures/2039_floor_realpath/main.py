# realpath resolves a symlink chain through posix.readlink. The fixture runs in
# its own throwaway directory, so it makes a real file and a symlink to it and
# checks the host-invariant relationships: the link resolves to the same place
# the target does, islink tells them apart, and readlink hands back the target
# name. The absolute paths themselves embed the temp directory, so they are
# never printed.
import os
import os.path

# The builtin open() is not up yet (a later slice), so the target file is made
# with the os-level fd calls that already exist.
fd = os.open("target.txt", os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o644)
os.write(fd, b"hi")
os.close(fd)
os.symlink("target.txt", "link.txt")

print("islink_link", os.path.islink("link.txt"))
print("islink_target", os.path.islink("target.txt"))
print("readlink", os.readlink("link.txt"))
print("realpath_matches", os.path.realpath("link.txt") == os.path.realpath("target.txt"))
print("realpath_abs", os.path.realpath("link.txt").startswith("/"))
print("realpath_basename", os.path.basename(os.path.realpath("link.txt")))
print("exists_via_link", os.path.exists("link.txt"))

# A broken link still reports as a link but does not exist.
os.symlink("no_such_target_xyzzy", "broken.txt")
print("broken_islink", os.path.islink("broken.txt"))
print("broken_exists", os.path.exists("broken.txt"))
