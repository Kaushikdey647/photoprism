/*
Package cull detects near-duplicate photo bursts and applies automatic cull groups.

Copyright (c) 2018 - 2026 PhotoPrism UG. All rights reserved.

	This program is free software: you can redistribute it and/or modify
	it under Version 3 of the GNU Affero General Public License (the "AGPL"):
	<https://docs.photoprism.app/license/agpl>

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	The AGPL is supplemented by our Trademark and Brand Guidelines,
	which describe how our Brand Assets may be used:
	<https://www.photoprism.app/trademark/>
*/
package cull

import (
	"math"
	"math/bits"
	"sort"
	"time"
)

// Options configures near-duplicate cull detection and auto-archive behavior.
type Options struct {
	Window       time.Duration // Max taken_at gap within a time cluster.
	MaxDiff      int           // Max Hamming distance between primary FileDiff values.
	SameCamera   bool          // Require matching camera_id.
	MinSize      int           // Minimum group size (default 2).
	AutoArchive  bool          // Soft-archive reject members.
	SkipFavorite bool          // Never auto-archive favorites.
	Force        bool          // Rebuild unreviewed auto groups for already-cullled photos when forced.
}

// DefaultOptions returns safe automatic cull defaults.
func DefaultOptions() Options {
	return Options{
		Window:       2 * time.Second,
		MaxDiff:      3,
		SameCamera:   true,
		MinSize:      2,
		AutoArchive:  true,
		SkipFavorite: true,
	}
}

// Candidate is a photo considered for cull grouping.
type Candidate struct {
	PhotoUID    string
	TakenAt     time.Time
	CameraID    uint
	FileDiff    int
	Quality     int
	Resolution  int
	Favorite    bool
	Sharpness   float64 // Optional Laplacian variance; 0 if unknown.
	AlreadyCull bool
}

// Member is a ranked candidate inside a proposed cull group.
type Member struct {
	Candidate
	RankScore float64
	Role      string
}

// Group is a proposed near-duplicate cull set.
type Group struct {
	Members   []Member
	KeeperUID string
	Score     float64
}

// DiffDistance returns the Hamming distance between two FileDiff integers.
func DiffDistance(a, b int) int {
	return bits.OnesCount64(uint64(uint32(a) ^ uint32(b)))
}

// Similar reports whether two candidates are near-duplicates under opt.
func Similar(a, b Candidate, opt Options) bool {
	if opt.SameCamera && a.CameraID != b.CameraID {
		return false
	}

	if a.FileDiff == 0 || b.FileDiff == 0 {
		return false
	}

	return DiffDistance(a.FileDiff, b.FileDiff) <= opt.MaxDiff
}

// RankScore computes a keeper score from quality, resolution, and sharpness.
func RankScore(c Candidate) float64 {
	quality := float64(c.Quality)
	if quality < 0 {
		quality = 0
	}

	res := float64(c.Resolution)
	if res < 0 {
		res = 0
	}

	sharp := c.Sharpness
	if sharp < 0 {
		sharp = 0
	}

	// Normalize sharpness with a soft cap so huge values do not dominate.
	sharpNorm := math.Min(sharp/500.0, 10.0)

	return quality*10.0 + res + sharpNorm
}

// ClusterGroups builds cull groups from sorted candidates using time and FileDiff gates.
func ClusterGroups(candidates []Candidate, opt Options) []Group {
	if opt.MinSize < 2 {
		opt.MinSize = 2
	}

	if opt.Window <= 0 {
		opt.Window = 2 * time.Second
	}

	if len(candidates) < opt.MinSize {
		return nil
	}

	sorted := make([]Candidate, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TakenAt.Equal(sorted[j].TakenAt) {
			return sorted[i].PhotoUID < sorted[j].PhotoUID
		}
		return sorted[i].TakenAt.Before(sorted[j].TakenAt)
	})

	used := make(map[string]bool, len(sorted))
	groups := make([]Group, 0)

	for i := 0; i < len(sorted); i++ {
		seed := sorted[i]

		if used[seed.PhotoUID] || (!opt.Force && seed.AlreadyCull) {
			continue
		}

		cluster := []Candidate{seed}

		for j := i + 1; j < len(sorted); j++ {
			other := sorted[j]

			if used[other.PhotoUID] || (!opt.Force && other.AlreadyCull) {
				continue
			}

			if other.TakenAt.Sub(seed.TakenAt) > opt.Window {
				break
			}

			// Must be similar to the seed and within window of every current member's seed time.
			if !Similar(seed, other, opt) {
				continue
			}

			// Also require adjacency within window of the previous accepted member.
			prev := cluster[len(cluster)-1]
			if other.TakenAt.Sub(prev.TakenAt) > opt.Window {
				continue
			}

			if opt.SameCamera && other.CameraID != seed.CameraID {
				continue
			}

			cluster = append(cluster, other)
		}

		if len(cluster) < opt.MinSize {
			continue
		}

		// Expand: include any remaining candidates similar to any cluster member within window of seed.
		changed := true
		for changed {
			changed = false
			for j := i + 1; j < len(sorted); j++ {
				other := sorted[j]
				if used[other.PhotoUID] || (!opt.Force && other.AlreadyCull) {
					continue
				}
				if containsUID(cluster, other.PhotoUID) {
					continue
				}
				if other.TakenAt.Sub(seed.TakenAt) > opt.Window*time.Duration(len(cluster)) &&
					other.TakenAt.Sub(cluster[len(cluster)-1].TakenAt) > opt.Window {
					if other.TakenAt.Sub(seed.TakenAt) > opt.Window*3 {
						break
					}
					continue
				}

				ok := false
				for _, m := range cluster {
					if Similar(m, other, opt) && absDuration(other.TakenAt.Sub(m.TakenAt)) <= opt.Window {
						ok = true
						break
					}
				}
				if !ok {
					continue
				}
				cluster = append(cluster, other)
				changed = true
			}
		}

		if len(cluster) < opt.MinSize {
			continue
		}

		group := rankGroup(cluster)
		for _, m := range group.Members {
			used[m.PhotoUID] = true
		}
		groups = append(groups, group)
	}

	return groups
}

// containsUID reports whether candidates already include photoUID.
func containsUID(candidates []Candidate, photoUID string) bool {
	for _, c := range candidates {
		if c.PhotoUID == photoUID {
			return true
		}
	}
	return false
}

// absDuration returns the absolute value of a duration.
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// rankGroup assigns roles and scores to a candidate cluster.
func rankGroup(candidates []Candidate) Group {
	members := make([]Member, 0, len(candidates))

	for _, c := range candidates {
		members = append(members, Member{
			Candidate: c,
			RankScore: RankScore(c),
		})
	}

	sort.SliceStable(members, func(i, j int) bool {
		if members[i].RankScore == members[j].RankScore {
			return members[i].PhotoUID < members[j].PhotoUID
		}
		return members[i].RankScore > members[j].RankScore
	})

	for i := range members {
		if i == 0 {
			members[i].Role = "keeper"
		} else {
			members[i].Role = "reject"
		}
	}

	return Group{
		Members:   members,
		KeeperUID: members[0].PhotoUID,
		Score:     members[0].RankScore,
	}
}
