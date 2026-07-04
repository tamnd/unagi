# An unhandled missing-attribute access unwinds through the method that
# raised it, so the traceback names both frames.
class Widget:
    def __init__(self):
        self.width = 10

    def area(self):
        return self.width * self.height


w = Widget()
print(w.width)
print(w.area())
