// Command yomi reads a web page, or a whole website, into clean Markdown. It
// renders JavaScript when a page needs it, extracts the main content, and
// converts what is left to Markdown, either one file per page in a folder or a
// single combined document.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/tamnd/yomi/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		stop()
	}()

	os.Exit(cli.Execute(ctx))
}
