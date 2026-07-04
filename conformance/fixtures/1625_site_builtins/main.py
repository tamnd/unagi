def show(label, obj):
    print(label, repr(obj))

show("exit", exit)
show("quit", quit)
show("copyright", copyright)
show("credits", credits)
show("license", license)
show("help", help)

print(type(exit).__name__, type(quit).__name__)
print(type(copyright).__name__, type(credits).__name__, type(license).__name__)
print(type(help).__name__)

copyright()
print()
credits()
print()

for maker in (exit, quit):
    try:
        maker(7)
    except SystemExit as e:
        print(type(e).__name__, e.code, e.args)
    try:
        maker()
    except SystemExit as e:
        print(repr(e.code), e.args)
    try:
        maker("bye")
    except SystemExit as e:
        print(repr(e.code))

exit(3)
