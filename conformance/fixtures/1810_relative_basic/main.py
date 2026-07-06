# Relative imports resolve against the importer's package: one dot is the
# package itself, two walk to the parent, and `from . import name` takes the
# attribute or falls back to the sibling submodule.
import pkg.mod
import pkg.sub.leaf

import pkg

print(pkg.mod.sib is pkg.sib)
print(pkg.sub.leaf.marker)
