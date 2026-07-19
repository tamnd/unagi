# expect: UNA-THR-001
# A read-modify-write of a shared global from a thread loses updates.
import threading

counter = 0


def worker():
    global counter
    for _ in range(100000):
        counter += 1


threading.Thread(target=worker).start()
