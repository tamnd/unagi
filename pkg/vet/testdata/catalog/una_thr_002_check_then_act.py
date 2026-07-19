# expect: UNA-THR-002
# Two threads can both pass the check and both build the entry.
import threading

cache = {}


def get(key):
    if key not in cache:
        cache[key] = build(key)
    return cache[key]


threading.Thread(target=get, args=("k",)).start()
