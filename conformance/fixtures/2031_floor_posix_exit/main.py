# posix._exit ends the process now with the given status and skips the normal
# interpreter teardown, so it does not flush buffered output. The line below is
# flushed explicitly so it survives regardless of the stdout buffering model,
# and the line after _exit never runs.
import posix

print("before", flush=True)
posix._exit(2)
print("after")
