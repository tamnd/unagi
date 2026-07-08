# globals() hands back the one live module namespace: it keeps its identity
# across calls, and a write to it carries into the module storage so an
# injected name is visible to a later read of the namespace. This is the write
# side re._constants leans on when _makecodes runs globals().update(...).

print(type(globals()) is dict)
print(globals() is globals())

# Inject names through update, the _makecodes shape.
globals().update({'ALPHA': 0, 'BETA': 1, 'GAMMA': 2})
print(globals()['ALPHA'], globals()['BETA'], globals()['GAMMA'])
print('BETA' in globals())
print(sorted(k for k in globals() if k in ('ALPHA', 'BETA', 'GAMMA')))

# A single-key write through subscription carries back too, and the same dict
# object captured earlier sees it because globals() is stable.
g = globals()
g['DELTA'] = 99
print(globals()['DELTA'])
print('DELTA' in g)

# Rebinding an existing module global through globals() updates the live value
# a bare read returns.
COUNT = 1
globals()['COUNT'] = 5
print(COUNT)
g['COUNT'] = 9
print(COUNT)

# Deleting an injected name through globals() unbinds it from the namespace.
del g['DELTA']
print('DELTA' in globals())

# The module's own identity attributes are part of the namespace.
print(globals()['__name__'])
