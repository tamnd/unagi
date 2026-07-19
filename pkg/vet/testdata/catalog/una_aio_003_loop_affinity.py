# expect: UNA-AIO-003
# A worker thread touches the loop directly instead of call_soon_threadsafe.
import threading


def worker(loop, fut):
    loop.call_soon(fut.set_result, compute())


threading.Thread(target=worker, args=(loop, fut)).start()
