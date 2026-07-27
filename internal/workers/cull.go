package workers

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/dustin/go-humanize/english"

	"github.com/photoprism/photoprism/internal/config"
	"github.com/photoprism/photoprism/internal/entity"
	"github.com/photoprism/photoprism/internal/mutex"
	"github.com/photoprism/photoprism/internal/photoprism/cull"
	"github.com/photoprism/photoprism/internal/thumb"
)

// Cull detects near-duplicate bursts and applies automatic cull groups.
type Cull struct {
	conf *config.Config
}

// NewCull constructs a Cull worker bound to the provided configuration.
func NewCull(conf *config.Config) *Cull {
	return &Cull{conf: conf}
}

// StartScheduled executes the cull worker on the cron schedule.
func (w *Cull) StartScheduled() {
	if w.conf == nil || !w.conf.CullEnabled() {
		return
	}

	if err := w.Start(0, false); err != nil {
		log.Errorf("scheduler: %s (cull)", err)
	}
}

// Start runs near-duplicate detection and optional auto-archive.
func (w *Cull) Start(limit int, force bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cull: %s (worker panic)\nstack: %s", r, debug.Stack())
			log.Error(err)
		}
	}()

	if w.conf == nil || !w.conf.CullEnabled() {
		log.Infof("cull: disabled")
		return nil
	}

	if err = mutex.CullWorker.Start(); err != nil {
		return err
	}
	defer mutex.CullWorker.Stop()

	opt := w.conf.CullOptions()
	opt.Force = force

	if limit <= 0 {
		limit = 50000
	}

	candidates, err := cull.LoadCandidates(limit, time.Time{})
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		log.Infof("cull: no candidates found")
		return nil
	}

	thumbPath := w.conf.ThumbPath()
	size := thumb.SizeTile224

	for i := range candidates {
		file, fileErr := entity.PrimaryFile(candidates[i].PhotoUID)
		if fileErr != nil || file == nil || file.FileHash == "" || len(file.FileHash) < 2 {
			continue
		}

		name, nameErr := size.FileName(file.FileHash, thumbPath)
		if nameErr != nil {
			name = filepath.Join(thumbPath, file.FileHash[:1], file.FileHash[1:2], file.FileHash+"_224x224_center.jpg")
		}

		candidates[i].Sharpness = cull.LaplacianVariance(name)
	}

	groups := cull.ClusterGroups(candidates, opt)
	if len(groups) == 0 {
		log.Infof("cull: no near-duplicate groups found among %s", english.Plural(len(candidates), "candidate", "candidates"))
		return nil
	}

	result, err := cull.Apply(groups, opt)
	if err != nil {
		return err
	}

	log.Infof("cull: created %s, archived %s, skipped %s",
		english.Plural(result.GroupsCreated, "group", "groups"),
		english.Plural(result.PhotosArchived, "photo", "photos"),
		english.Plural(result.PhotosSkipped, "photo", "photos"),
	)

	return nil
}
