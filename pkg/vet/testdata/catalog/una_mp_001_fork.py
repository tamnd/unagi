# expect: UNA-MP-001
# The fork start method is unsupported; unagi workers are spawn-only.
import multiprocessing

multiprocessing.set_start_method("fork")
