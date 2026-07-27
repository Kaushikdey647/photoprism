package config

import (
	"testing"
	"time"
)

func TestConfig_CullOptions(t *testing.T) {
	c := NewMinimalTestConfig(t.TempDir())

	if !c.CullEnabled() {
		t.Fatal("expected cull enabled by default")
	}
	if c.CullWindow() != 2*time.Second {
		t.Fatalf("window = %v, want 2s", c.CullWindow())
	}
	if c.CullDiff() != 3 {
		t.Fatalf("diff = %d, want 3", c.CullDiff())
	}
	if !c.CullSameCamera() {
		t.Fatal("expected same camera default true")
	}
	if !c.CullAutoArchive() {
		t.Fatal("expected auto archive default true")
	}
	if !c.CullSkipFavorites() {
		t.Fatal("expected skip favorites default true")
	}

	c.options.CullDisabled = true
	if c.CullEnabled() {
		t.Fatal("expected cull disabled")
	}
	if c.CullSchedule() != "" {
		t.Fatal("expected empty schedule when disabled")
	}
}

func TestConfig_CullWindowFromSettings(t *testing.T) {
	c := NewMinimalTestConfig(t.TempDir())
	c.Settings().Cull.Window = 5

	if c.CullWindow() != 5*time.Second {
		t.Fatalf("window = %v, want 5s", c.CullWindow())
	}
}
