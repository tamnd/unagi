# expect: UNA-THR-003
# The consumer walks items while a producer thread appends to it.
import threading

items = []


def consume():
    for x in items:
        handle(x)


def produce():
    items.append(make())


threading.Thread(target=produce).start()
consume()
