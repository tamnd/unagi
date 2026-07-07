import sys

class Repl:
    tag = "replaced sub"

sys.modules[__name__] = Repl()
