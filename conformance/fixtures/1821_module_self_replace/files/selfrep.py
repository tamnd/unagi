import sys

class API:
    tag = "replacement"

    def hello(self):
        return "hello from replacement"

value = "module level"
sys.modules[__name__] = API()
