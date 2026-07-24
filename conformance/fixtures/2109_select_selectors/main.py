# The native select accelerator exposes select.select, the one primitive POSIX
# guarantees everywhere, so selectors resolves to SelectSelector on every host.
# The test uses os.pipe fds, whose readiness is deterministic, and compares fds
# against the local variables rather than printing the raw numbers.
import os
import select
import selectors

# A written pipe's read end is ready; the empty write and exceptional sets stay
# empty at a zero timeout.
r, w = os.pipe()
os.write(w, b"x")
rr, ww, xx = select.select([r], [], [], 0)
print(rr == [r], ww, xx)

# With nothing written, a zero timeout returns all three sets empty.
r2, w2 = os.pipe()
rr, ww, xx = select.select([r2], [], [], 0)
print(rr, ww, xx)

# selectors imports and its portable SelectSelector reports the registration:
# the key carries the fd, the attached data, and the read event mask.
sel = selectors.SelectSelector()
key = sel.register(r, selectors.EVENT_READ, "payload")
print(key.fd == r)
print(key.data)
events = sel.select(timeout=0)
print(len(events))
evkey, mask = events[0]
print(evkey.fd == r)
print(mask == selectors.EVENT_READ)

# get_key round-trips the registration and unregister drops it.
print(sel.get_key(r).data)
sel.unregister(r)
print(len(sel.get_map()))
sel.close()

# The SelectorKey namedtuple carries the fuller __doc__ selectors assigns it.
print(selectors.SelectorKey.__doc__.splitlines()[0])
print(selectors.SelectorKey._fields)

for fd in (r, w, r2, w2):
    os.close(fd)
