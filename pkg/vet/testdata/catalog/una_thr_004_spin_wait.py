# expect: UNA-THR-004
# A busy loop polls an unsynchronized flag another thread flips.
import threading

done = False


def worker():
    global done
    run()
    done = True


threading.Thread(target=worker).start()
while not done:
    pass
