package resolve

import "strings"

import "testing"

func TestManifestParses(t *testing.T) {
	if len(manifest.Images) == 0 {
		t.Fatal("embedded manifest.lock has no images")
	}
	if manifest.Tools["jre"] != "11" {
		t.Errorf("expected pinned JRE 11, got %q", manifest.Tools["jre"])
	}
}

func TestImage_DigestPinned(t *testing.T) {
	ref, pinned := Image("kics", "")
	if !pinned {
		t.Errorf("kics should resolve to a pinned digest, got %q", ref)
	}
	if !strings.Contains(ref, "@sha256:") {
		t.Errorf("expected digest reference, got %q", ref)
	}
	if !strings.HasPrefix(ref, "checkmarx/kics@sha256:") {
		t.Errorf("unexpected ref %q", ref)
	}
}

func TestImage_OverrideWins(t *testing.T) {
	ref, pinned := Image("kics", "myregistry/kics@sha256:abc")
	if ref != "myregistry/kics@sha256:abc" {
		t.Errorf("override should win, got %q", ref)
	}
	if !pinned {
		t.Errorf("override with @digest should be considered pinned")
	}
	ref, pinned = Image("kics", "myregistry/kics:dev")
	if pinned {
		t.Errorf("override by tag should be reported as unpinned")
	}
	_ = ref
}

func TestImage_UnknownEngine(t *testing.T) {
	if ref, _ := Image("nope", ""); ref != "" {
		t.Errorf("unknown engine should return empty, got %q", ref)
	}
}
