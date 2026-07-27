package config

import (
	"strings"
	"time"

	"github.com/photoprism/photoprism/internal/photoprism/cull"
)

// CullEnabled reports whether near-duplicate cull detection is enabled.
func (c *Config) CullEnabled() bool {
	if c == nil {
		return false
	}

	if c.options.CullDisabled {
		return false
	}

	return c.Settings().Cull.Enabled
}

// CullSchedule returns the cron schedule for the cull worker, or "" if disabled.
func (c *Config) CullSchedule() string {
	if c == nil || !c.CullEnabled() {
		return ""
	}

	return Schedule(c.options.CullSchedule)
}

// CullFilter returns the search filter used for scheduled cull runs.
func (c *Config) CullFilter() string {
	if c == nil {
		return ""
	}

	return strings.TrimSpace(c.options.CullFilter)
}

// CullWindow returns the maximum taken-time gap for burst grouping.
func (c *Config) CullWindow() time.Duration {
	if c == nil {
		return 2 * time.Second
	}

	if w := c.Settings().Cull.Window; w > 0 {
		return time.Duration(w) * time.Second
	}

	if c.options.CullWindow <= 0 {
		return 2 * time.Second
	}

	return time.Duration(c.options.CullWindow) * time.Second
}

// CullDiff returns the maximum FileDiff Hamming distance for near-duplicates.
func (c *Config) CullDiff() int {
	if c == nil || c.options.CullDiff <= 0 {
		return 3
	}

	return c.options.CullDiff
}

// CullSameCamera reports whether grouping requires the same camera.
func (c *Config) CullSameCamera() bool {
	if c == nil {
		return true
	}

	return c.Settings().Cull.SameCamera
}

// CullMin returns the minimum group size for a cull.
func (c *Config) CullMin() int {
	if c == nil || c.options.CullMin < 2 {
		return 2
	}

	return c.options.CullMin
}

// CullAutoArchive reports whether non-keepers are soft-archived automatically.
func (c *Config) CullAutoArchive() bool {
	if c == nil {
		return true
	}

	return c.Settings().Cull.AutoArchive
}

// CullSkipFavorites reports whether favorites are protected from auto-archive.
func (c *Config) CullSkipFavorites() bool {
	if c == nil {
		return true
	}

	return c.options.CullSkipFavorites
}

// CullOptions returns runtime options for the cull package.
func (c *Config) CullOptions() cull.Options {
	opt := cull.DefaultOptions()

	if c == nil {
		return opt
	}

	opt.Window = c.CullWindow()
	opt.MaxDiff = c.CullDiff()
	opt.SameCamera = c.CullSameCamera()
	opt.MinSize = c.CullMin()
	opt.AutoArchive = c.CullAutoArchive()
	opt.SkipFavorite = c.CullSkipFavorites()

	return opt
}
