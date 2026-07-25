import subprocess

# A nonzero exit is reported on returncode.
r = subprocess.run(["sh", "-c", "exit 3"])
print("rc", r.returncode)

# Text input is fed to the child's stdin and its stdout captured back.
r = subprocess.run(["cat"], input="piped\n", capture_output=True, text=True)
print("cat", repr(r.stdout))

# check_output returns just the captured stdout.
out = subprocess.check_output(["echo", "-n", "xy"], text=True)
print("out", repr(out))

# check=True turns a nonzero exit into CalledProcessError carrying the code.
try:
    subprocess.run(["false"], check=True)
except subprocess.CalledProcessError as e:
    print("called", e.returncode)

# A missing executable raises FileNotFoundError, the way the errno maps back
# through fork_exec.
try:
    subprocess.run(["no_such_cmd_xyz_123"])
except FileNotFoundError as e:
    print("fnf", type(e).__name__)

# Popen with two pipes plus communicate round-trips data through a real child.
p = subprocess.Popen(["sort"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
o, _ = p.communicate("b\na\nc\n")
print("sort", repr(o), p.returncode)
