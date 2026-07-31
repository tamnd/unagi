import os
import sys
import gc

# Clear the colour environment so the result depends only on whether stdout is a
# terminal, which is host-invariant under the conformance harness (a pipe, never
# a tty), keeping the printed value identical on the record and replay hosts.
for _var in ("PYTHON_COLORS", "FORCE_COLOR", "NO_COLOR"):
    os.environ.pop(_var, None)

from _colorize import can_colorize

# can_colorize probes the stream's file descriptor to decide whether it is a
# terminal. The probe must inspect the borrowed descriptor without taking
# ownership of it: a probe that wrapped the fd in a file whose finaliser closed
# it would break the very stream being probed, so a later write to stdout would
# fail once a garbage collection ran. Probe stdout repeatedly, force a
# collection, then keep writing to prove the descriptor stayed open.
results = [can_colorize(file=sys.stdout) for _ in range(8)]
gc.collect()

print("can_colorize:", results[0])
print("stable:", all(r == results[0] for r in results))

# These writes happen after the probes and the collection; on the historical bug
# they raised OSError (bad file descriptor) instead of printing.
for i in range(3):
    print("write after probe", i)

print("done")
