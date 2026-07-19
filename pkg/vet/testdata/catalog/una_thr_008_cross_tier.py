# expect: UNA-THR-008
# A type-annotated global rebound from a thread can read a stale typed shadow.
import threading

counter: int = 0


def reset():
    global counter
    counter = fresh()


threading.Thread(target=reset).start()
