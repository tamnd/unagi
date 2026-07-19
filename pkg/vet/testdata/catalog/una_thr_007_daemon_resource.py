# expect: UNA-THR-007
# A daemon thread writes a file; shutdown kills it without flushing.
import threading


def log_forever():
    with open("out.log", "a") as f:
        while True:
            f.write(next_line())


threading.Thread(target=log_forever, daemon=True).start()
