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
func exitCode(err error) int {
	switch xiaohongshu.Kind(err) {
	case xiaohongshu.ErrNotFound:
		return 4
	case xiaohongshu.ErrAccess:
		return 3
	case xiaohongshu.ErrRate, xiaohongshu.ErrAntibot:
		return 5
	case xiaohongshu.ErrNetwork:
		return 6
	default:
		return 1
	}
}
