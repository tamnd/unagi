// Command unagi is a Python to Go compiler. It compiles Python source into
// readable Go and builds a single static binary, no CPython and no cgo in the
// result, and it will package Go libraries as Python wheels for the trip in
// the other direction.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/tamnd/unagi/pkg/cli"
)

func main() {
	// A signal-aware context so Ctrl-C and SIGTERM cancel a running compile or
	// program instead of leaving stray build directories behind.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Execute(ctx))
}
