# expect: UNA-MP-002
# A lambda sent to a worker process cannot be pickled by qualified name.
import multiprocessing

multiprocessing.Process(target=lambda: work()).start()
