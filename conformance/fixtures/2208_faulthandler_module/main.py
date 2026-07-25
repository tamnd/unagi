import faulthandler

# The observable contract is the enabled state.
print("initial:", faulthandler.is_enabled())
faulthandler.enable()
print("after enable:", faulthandler.is_enabled())
faulthandler.disable()
print("after disable:", faulthandler.is_enabled())

# enable() is idempotent and re-enables after a disable.
faulthandler.enable()
faulthandler.enable()
print("re-enabled:", faulthandler.is_enabled())
faulthandler.disable()
print("final:", faulthandler.is_enabled())

# The scheduling entry points accept their arguments and return None. The
# traceback-dumping calls (dump_traceback, dump_c_stack) write to stderr, so they
# are left out here to keep the diff host-invariant; the unit test covers them.
# dump_traceback_later(60) arms a watchdog that fires long after the program
# exits, so it dumps nothing.
print("dump_later:", faulthandler.dump_traceback_later(60))
print("cancel_later:", faulthandler.cancel_dump_traceback_later())

# register()/unregister() manage a per-signal handler; unregister reports
# whether one was present.
import signal
print("register:", faulthandler.register(signal.SIGUSR1))
print("unregister:", faulthandler.unregister(signal.SIGUSR2))

# dump_traceback_later rejects a non-numeric timeout.
try:
    faulthandler.dump_traceback_later("soon")
except TypeError:
    print("bad timeout: TypeError")
