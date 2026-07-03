# unagi (鰻)

Compile Python to Go, ship Go to Python.

unagi is a Python compiler with two directions.
`unagi build` compiles a Python program into readable Go and produces a single static binary, no CPython and no cgo anywhere in the result.
`unagi wheel` will go the other way, packaging a Go library as a standard Python wheel.

The eel swims both directions.

## Status

Pre-alpha, under heavy construction.
The current build carries the M0 skeleton: a Python frontend, a boxed object runtime, and a build pipeline that turns a Python file into a running Go binary for a growing subset of the language.
Nothing here is ready for real programs yet.

## How it works

The compiler parses Python 3.14 source, resolves scopes, and lowers the program onto a Go runtime that mirrors CPython's object semantics: real ints with arbitrary precision to come, ordered dicts, Python truthiness, Python exceptions.
Code that type analysis can prove will later lower to plain unboxed Go, with guards that fall back to the boxed path instead of breaking; that mixed-execution design is specified but not built yet.
CPython 3.14 is the semantic oracle: where behavior is observable, unagi matches it, and every deliberate divergence is documented.

## Try it

```sh
go install github.com/tamnd/unagi/cmd/unagi@latest

echo 'print("hello from the eel")' > hello.py
unagi run hello.py
unagi build hello.py -o hello && ./hello
```

`unagi build --emit-go` writes the generated Go package out so you can read what your Python became.

## License

MIT
