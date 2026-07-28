# curses imports through the reduced-surface _curses accelerator: its attribute,
# color, and key constants are all real, so the module is usable for the constant
# surface even though this build carries no terminal to draw on.
import curses

print("name", curses.__name__)

# The ncurses attribute bits, color ids, and key codes come from ncurses'
# headers, so they are fixed.
names = [
    "A_NORMAL", "A_BOLD", "A_UNDERLINE", "A_REVERSE", "A_BLINK", "A_DIM",
    "COLOR_BLACK", "COLOR_RED", "COLOR_GREEN", "COLOR_YELLOW",
    "COLOR_BLUE", "COLOR_MAGENTA", "COLOR_CYAN", "COLOR_WHITE",
    "KEY_UP", "KEY_DOWN", "KEY_LEFT", "KEY_RIGHT", "KEY_HOME", "KEY_END",
    "KEY_BACKSPACE", "KEY_ENTER", "KEY_F0",
    "ERR", "OK",
]
for n in names:
    print(n, getattr(curses, n))

# error is a real catchable exception, and window is a type.
print("error<-Exception", issubclass(curses.error, Exception))
print("window is type", isinstance(curses.window, type))

# has_key is provided by curses's own pure fallback when _curses omits it.
print("has_key present", hasattr(curses, "has_key"))
