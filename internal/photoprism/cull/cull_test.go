package cull

import (
	"testing"
	"time"
)

func TestDiffDistance(t *testing.T) {
	if got := DiffDistance(42, 42); got != 0 {
		t.Fatalf("DiffDistance(42,42) = %d, want 0", got)
	}
	if got := DiffDistance(0b0010, 0b0000); got != 1 {
		t.Fatalf("DiffDistance bit flip = %d, want 1", got)
	}
	if got := DiffDistance(0b1111, 0b0000); got < 4 {
		t.Fatalf("DiffDistance(0b1111,0) = %d, want >= 4", got)
	}
}

func TestRankScore(t *testing.T) {
	low := RankScore(Candidate{Quality: 1, Resolution: 1, Sharpness: 0})
	high := RankScore(Candidate{Quality: 5, Resolution: 12, Sharpness: 800})
	if high <= low {
		t.Fatalf("expected high score %f > low score %f", high, low)
	}
}

func TestClusterGroups(t *testing.T) {
	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	opt := DefaultOptions()

	t.Run("TimeAndDiff", func(t *testing.T) {
		candidates := []Candidate{
			{PhotoUID: "p1", TakenAt: base, CameraID: 1, FileDiff: 100, Quality: 3, Resolution: 12},
			{PhotoUID: "p2", TakenAt: base.Add(500 * time.Millisecond), CameraID: 1, FileDiff: 100, Quality: 4, Resolution: 12},
			{PhotoUID: "p3", TakenAt: base.Add(10 * time.Second), CameraID: 1, FileDiff: 100, Quality: 5, Resolution: 12},
		}

		groups := ClusterGroups(candidates, opt)
		if len(groups) != 1 {
			t.Fatalf("got %d groups, want 1", len(groups))
		}
		if groups[0].KeeperUID != "p2" {
			t.Fatalf("keeper = %s, want p2", groups[0].KeeperUID)
		}
		if len(groups[0].Members) != 2 {
			t.Fatalf("members = %d, want 2", len(groups[0].Members))
		}
	})

	t.Run("SameCameraRequired", func(t *testing.T) {
		candidates := []Candidate{
			{PhotoUID: "p1", TakenAt: base, CameraID: 1, FileDiff: 100, Quality: 3, Resolution: 12},
			{PhotoUID: "p2", TakenAt: base.Add(200 * time.Millisecond), CameraID: 2, FileDiff: 100, Quality: 5, Resolution: 12},
		}

		groups := ClusterGroups(candidates, opt)
		if len(groups) != 0 {
			t.Fatalf("got %d groups, want 0", len(groups))
		}
	})

	t.Run("DiffThreshold", func(t *testing.T) {
		candidates := []Candidate{
			{PhotoUID: "p1", TakenAt: base, CameraID: 1, FileDiff: 0b00000001, Quality: 3, Resolution: 12},
			{PhotoUID: "p2", TakenAt: base.Add(200 * time.Millisecond), CameraID: 1, FileDiff: 0b11111110, Quality: 5, Resolution: 12},
		}

		groups := ClusterGroups(candidates, opt)
		if len(groups) != 0 {
			t.Fatalf("got %d groups, want 0", len(groups))
		}
	})

	t.Run("SkipAlreadyCulled", func(t *testing.T) {
		candidates := []Candidate{
			{PhotoUID: "p1", TakenAt: base, CameraID: 1, FileDiff: 100, Quality: 3, Resolution: 12, AlreadyCull: true},
			{PhotoUID: "p2", TakenAt: base.Add(200 * time.Millisecond), CameraID: 1, FileDiff: 100, Quality: 5, Resolution: 12},
		}

		groups := ClusterGroups(candidates, opt)
		if len(groups) != 0 {
			t.Fatalf("got %d groups, want 0", len(groups))
		}
	})
}

func TestSimilar(t *testing.T) {
	opt := DefaultOptions()
	a := Candidate{CameraID: 1, FileDiff: 10}
	b := Candidate{CameraID: 1, FileDiff: 10}
	if !Similar(a, b, opt) {
		t.Fatal("expected similar candidates")
	}

	b.FileDiff = 0
	if Similar(a, b, opt) {
		t.Fatal("zero FileDiff must not be similar")
	}
}
