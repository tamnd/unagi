# expect: UNA-THR-005
# The lock is acquired by hand, and a raise between acquire and release leaks it.
import threading

lock = threading.Lock()


def worker():
    lock.acquire()
    do_work()
    lock.release()


threading.Thread(target=worker).start()
