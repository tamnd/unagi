import mid

print(mid.TABLE)
print(mid.KEPT)
print(mid.total())
print(mid.ALPHA, mid.BETA, mid.GAMMA)
# The underscore names are not part of the star surface.
print(hasattr(mid, '_HIDDEN'))
print(hasattr(mid, '_makecodes'))
