import lib

# __dict__ is the live module namespace, an ordinary dict with every bound name.
d = lib.__dict__
print(type(d) is dict)
print(d['A'], d['B'])
print(d['_HIDDEN'])
print('total' in d, '__name__' in d)

# The same object comes back each read.
print(d is lib.__dict__)

# A write through __dict__ carries into the module, the enum.global_enum shape.
lib.__dict__.update({'C': 30, 'D': 40})
print(lib.C, lib.D)
lib.__dict__['E'] = 50
print(lib.E)

# The function still reads the module globals through the same namespace.
print(lib.total())
