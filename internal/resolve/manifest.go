// Package resolve resolves the engine tools cx-onprem-orchestrator drives. Its
// core security property is digest pinning: docker images are referenced by
// sha256 digest from the embedded manifest.lock, never by a mutable tag.
package resolve

import (
	_ "embed"
	"encoding/json"
)

//go:embed manifest.lock
var manifestBytes []byte

// ImagePin is one pinned docker image.
type ImagePin struct {
	Ref    string `json:"ref"`
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

// Manifest is the parsed manifest.lock.
type Manifest struct {
	Images map[string]ImagePin `json:"images"`
	Tools  map[string]string   `json:"tools"`
}

var manifest Manifest

func init() {
	// manifest.lock is embedded and validated at build time; a parse failure is a
	// programming error, so panic loudly rather than ship an unpinned binary.
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		panic("resolve: invalid embedded manifest.lock: " + err.Error())
	}
}

// Image returns the docker image reference for an engine token, applying the
// pinning policy:
//
//   - if override is non-empty, it wins (operator chose a specific image/digest);
//   - else if the manifest has a digest for the engine, return "ref@sha256:...";
//   - else fall back to "ref:tag" (and the caller should warn that it is unpinned).
func Image(engine, override string) (ref string, pinned bool) {
	if override != "" {
		return override, hasDigest(override)
	}
	p, ok := manifest.Images[engine]
	if !ok {
		return "", false
	}
	if p.Digest != "" {
		return p.Ref + "@" + p.Digest, true
	}
	return p.Ref + ":" + p.Tag, false
}

// ToolVersion returns the pinned version recorded for a non-docker tool.
func ToolVersion(name string) string { return manifest.Tools[name] }

func hasDigest(ref string) bool {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '@' {
			return true
		}
	}
	return false
}
