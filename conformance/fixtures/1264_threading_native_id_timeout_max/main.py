import threading

# TIMEOUT_MAX is a fixed float, the largest timeout the lock primitives accept.
print("timeout_max", threading.TIMEOUT_MAX)
print("timeout_max type", type(threading.TIMEOUT_MAX).__name__)

# get_native_id is a positive int, stable across calls within a thread.
nid = threading.get_native_id()
print("native_id type", type(nid).__name__)
print("native_id positive", nid > 0)
print("native_id stable", nid == threading.get_native_id())

# the main thread object reports the same value through its native_id attribute
print("matches attr", nid == threading.current_thread().native_id)
