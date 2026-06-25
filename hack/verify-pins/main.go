// Command verify-pins checks that the docker image digests pinned in
// internal/resolve/manifest.lock still match what each :tag currently resolves to
// in the registry. Run it as a bump-guard (CI + `make verify-pins`): a non-zero
// exit means an upstream tag was re-pushed to a new digest (as in the 2026-04
// checkmarx/kics repo compromise) and the pin must be re-reviewed before use.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type imagePin struct {
	Ref    string `json:"ref"`
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

type manifest struct {
	Images map[string]imagePin `json:"images"`
}

const manifestPath = "internal/resolve/manifest.lock"

func main() {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-pins: cannot read %s (run from repo root): %v\n", manifestPath, err)
		os.Exit(2)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "verify-pins: invalid manifest.lock: %v\n", err)
		os.Exit(2)
	}

	drift := false
	for name, p := range m.Images {
		if p.Digest == "" {
			fmt.Printf("SKIP  %-10s %s:%s (no digest pinned yet)\n", name, p.Ref, p.Tag)
			continue
		}
		current, err := currentDigest(p.Ref, p.Tag)
		if err != nil {
			fmt.Printf("ERROR %-10s %s:%s — %v\n", name, p.Ref, p.Tag, err)
			drift = true
			continue
		}
		if current != p.Digest {
			fmt.Printf("DRIFT %-10s %s:%s\n      pinned : %s\n      current: %s\n", name, p.Ref, p.Tag, p.Digest, current)
			drift = true
			continue
		}
		fmt.Printf("OK    %-10s %s:%s @ %s\n", name, p.Ref, p.Tag, short(p.Digest))
	}

	if drift {
		fmt.Fprintln(os.Stderr, "\nverify-pins: FAILED — a pinned digest drifted or could not be verified.")
		fmt.Fprintln(os.Stderr, "Re-review the new image, then update internal/resolve/manifest.lock + Dockerfile.fat.")
		os.Exit(1)
	}
	fmt.Println("\nverify-pins: all pinned digests match.")
}

// currentDigest asks the registry what <ref>:<tag> currently resolves to, without
// pulling the image. Uses docker buildx imagetools (registry-only).
func currentDigest(ref, tag string) (string, error) {
	out, err := exec.Command("docker", "buildx", "imagetools", "inspect",
		ref+":"+tag, "--format", "{{.Manifest.Digest}}").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker buildx imagetools inspect failed: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func short(d string) string {
	if len(d) > 19 {
		return d[:19] + "…"
	}
	return d
}
