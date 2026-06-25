package ci

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// GitIntrospect is the production GitFunc: it shells out to git in workdir to
// recover branch/commit/repo. Any failure yields empty strings (best-effort).
func GitIntrospect(workdir string) (branch, commit, repo string) {
	run := func(args ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", args...)
		if workdir != "" {
			cmd.Dir = workdir
		}
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	branch = run("rev-parse", "--abbrev-ref", "HEAD")
	commit = run("rev-parse", "HEAD")
	repo = run("config", "--get", "remote.origin.url")
	return branch, commit, repo
}
