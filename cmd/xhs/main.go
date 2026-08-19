// Command xhs is a single-binary command line for Xiaohongshu.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/fang"
	"github.com/tamnd/xiaohongshu-cli/cli"
	"github.com/tamnd/xiaohongshu-cli/xiaohongshu"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.Root()
	if err := fang.Execute(ctx, root,
		fang.WithVersion(cli.Version),
		fang.WithCommit(cli.Commit),
	); err != nil {
		os.Exit(exitCode(err))
	}
}

// exitCode maps an error to a stable shell exit code so scripts can tell a not
// found from an anti-bot wall from a transient network failure.
//
// The client now sorts responses into ten states, but this table still collapses
// them onto the five numbers v0.2.0 published, so upgrading does not silently
// change what a script sees. The widened table lands with the exit code work and
// gets its own release note.
func exitCode(err error) int {
	switch xiaohongshu.StatusOf(err) {
	case xiaohongshu.StatusNotFound, xiaohongshu.StatusGone:
		return 4
	case xiaohongshu.StatusLogin, xiaohongshu.StatusToken:
		return 3
	case xiaohongshu.StatusWalled, xiaohongshu.StatusAntibot:
		return 5
	case xiaohongshu.StatusNetwork:
		return 6
	default:
		return 1
	}
}
