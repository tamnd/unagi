# A list method reads back as a first-class callable, so items.append binds a
# value that calls the same as items.append(x). re/_parser hoists the read out
# of its inner loop, itemsappend = items.append, which is why import re needs
# it. int already binds its methods this way; list now matches.

items = []
itemsappend = items.append
itemsappend(1)
itemsappend(2)
items.append(3)
print(items)

# The bound read carries the receiver, so extend through it mutates the list.
ext = items.extend
ext([4, 5])
print(items)

# It is an ordinary builtin method value: its type and name read back.
print(type(items.append).__name__)
print(items.append.__name__)

# A no-argument method reads and calls the same as the direct form.
c = [3, 1, 2]
srt = c.sort
srt()
print(c)

# A name that is not a list method still raises AttributeError at the read.
try:
    items.nope
except AttributeError as e:
    print(e)
