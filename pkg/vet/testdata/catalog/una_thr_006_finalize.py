# expect: UNA-THR-006
# The file is opened and its last reference dropped, relying on prompt close.
import threading


def worker():
    data = open("input.txt").read()
    return data


threading.Thread(target=worker).start()
