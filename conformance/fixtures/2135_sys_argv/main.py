# sys.argv is the command-line argument vector. argv[0] is the program path,
# host-specific, so this checks the stable shape rather than its value: it is a
# mutable list of strings with at least the program name, and rewriting it in
# place works the way argparse and a program that edits sys.argv rely on.
import sys

print(isinstance(sys.argv, list))
print(len(sys.argv) >= 1)
print(all(isinstance(a, str) for a in sys.argv))
print(sys.argv is sys.argv)

sys.argv.append("appended")
print(sys.argv[-1])

sys.argv[:] = ["prog", "one", "two"]
print(sys.argv)
print(sys.argv[0])
print(sys.argv[1:])

# orig_argv holds the launch argument vector independently of sys.argv, so the
# rewrites above leave it a non-empty list of its own.
print(isinstance(sys.orig_argv, list))
print(len(sys.orig_argv) >= 1)
print(sys.orig_argv is sys.argv)
