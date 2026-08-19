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

// exitCodes maps each response state to a shell exit code. One state, one
// number, so a script can act on a refusal without parsing English.
//
// v0.2.0 published five numbers and collapsed states that call for opposite
// responses onto the same one. Rate limiting and an anti-bot refusal both exited
// 5, which is "sleep and try again" and "stop, nothing you do here will work"
// wearing the same number. A note missing its xsec_token exited 3, which told
// the caller to find a cookie for a problem no cookie solves.
//
// The numbers 2, 3, 5, 6 and 7 mean the same thing here as in bilibili-cli, so
// one wrapper script can drive both tools.
var exitCodes = map[xiaohongshu.Status]int{
	xiaohongshu.StatusOK:       0,
	xiaohongshu.StatusError:    1,
	xiaohongshu.StatusAntibot:  2,
	xiaohongshu.StatusEmpty:    3,
	xiaohongshu.StatusLogin:    4,
	xiaohongshu.StatusNetwork:  5,
	xiaohongshu.StatusWalled:   6,
	xiaohongshu.StatusNotFound: 7,
	xiaohongshu.StatusToken:    8,
	xiaohongshu.StatusGone:     9,
}

// exitCode maps an error to its state's exit code. A state with no entry exits
// 1, because an unclassified refusal is a bug report and not a category.
func exitCode(err error) int {
	if code, ok := exitCodes[xiaohongshu.StatusOf(err)]; ok {
		return code
	}
	return 1
}
