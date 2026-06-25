package exec

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

var dockerSeq uint64

// dockerContainerName derives a deterministic, unique container name from the
// invocation and injects a matching `--name` into the docker args if one is not
// already present. It returns the chosen name so the caller can `docker kill` it
// on cancellation (sending SIGKILL to the docker client does not stop the
// container it spawned).
func dockerContainerName(inv *model.Invocation) string {
	// Honor an explicit --name the caller already set.
	for i, a := range inv.Args {
		if a == "--name" && i+1 < len(inv.Args) {
			return inv.Args[i+1]
		}
		if strings.HasPrefix(a, "--name=") {
			return strings.TrimPrefix(a, "--name=")
		}
	}
	seq := atomic.AddUint64(&dockerSeq, 1)
	name := fmt.Sprintf("cxoo-%s-%d-%d", inv.Engine, os.Getpid(), seq)
	// Insert `--name <name>` right after the `run` subcommand if present, else prepend.
	for i, a := range inv.Args {
		if a == "run" {
			rest := append([]string{"--name", name}, inv.Args[i+1:]...)
			inv.Args = append(inv.Args[:i+1], rest...)
			return name
		}
	}
	inv.Args = append([]string{"--name", name}, inv.Args...)
	return name
}

// killDocker best-effort stops a container by name.
func killDocker(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = osexec.CommandContext(ctx, "docker", "kill", name).Run()
}
