# from posix import * inside an imported module must bind the builtin's public
# surface even though the compiler never saw an export list for it.
import probe

print(probe.check())
