package exec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	osexec "os/exec"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

// Result is the outcome of running one child process.
type Result struct {
	// ExitCode is the child's exit status. A non-zero value is normal data
	// (e.g. a threshold breach), NOT reflected in Err.
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	// Err is set only for execution-level failures: binary not found, spawn
	// failure, context cancellation/timeout. It is nil for a clean spawn that
	// exits non-zero.
	Err error
}

// Options configure how a child process is run.
type Options struct {
	// Display is where prefixed, redacted child output is mirrored (e.g. os.Stderr).
	// If nil, output is captured but not displayed.
	Display io.Writer
	// Prefix is prepended to each displayed line (e.g. "[kics] ").
	Prefix string
	// Redactor scrubs secrets from displayed output.
	Redactor Redactor
}

// Run executes the invocation, streaming and capturing stdout/stderr and
// returning the captured bytes plus the child exit code. The process is launched
// with os.Environ() plus inv.Env. On context cancellation the child is killed
// (and, for docker invocations, the container too — see killDocker).
func Run(ctx context.Context, inv *model.Invocation, opts Options) *Result {
	res := &Result{}

	cmd := osexec.CommandContext(ctx, inv.Path, inv.Args...)
	if inv.WorkDir != "" {
		cmd.Dir = inv.WorkDir
	}
	cmd.Env = append(os.Environ(), inv.Env...)

	var outCap, errCap bytes.Buffer
	outDisplay := newPrefixWriter(opts.Display, opts.Prefix, opts.Redactor)
	errDisplay := newPrefixWriter(opts.Display, opts.Prefix, opts.Redactor)
	cmd.Stdout = &teeCapture{capture: &outCap, display: outDisplay}
	cmd.Stderr = &teeCapture{capture: &errCap, display: errDisplay}

	// For docker runs, give the container a deterministic name so we can kill it
	// if the context is cancelled (killing the docker client does not stop it).
	containerName := ""
	if inv.UsesDocker {
		containerName = dockerContainerName(inv)
	}

	runErr := cmd.Run()
	outDisplay.flush()
	errDisplay.flush()
	res.Stdout = outCap.Bytes()
	res.Stderr = errCap.Bytes()

	if ctx.Err() != nil {
		if containerName != "" {
			killDocker(containerName)
		}
		res.Err = ctx.Err()
		res.ExitCode = interruptedExit
		return res
	}

	if runErr != nil {
		var ee *osexec.ExitError
		if errors.As(runErr, &ee) {
			// Clean spawn, non-zero exit: data, not an error.
			res.ExitCode = ee.ExitCode()
			return res
		}
		// Spawn/exec failure (e.g. binary not found).
		res.Err = runErr
		res.ExitCode = -1
		return res
	}

	res.ExitCode = 0
	return res
}

// interruptedExit mirrors the conventional 130 for interrupted processes.
const interruptedExit = 130
