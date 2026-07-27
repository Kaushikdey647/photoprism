package cull

import (
	"fmt"
	"time"

	"github.com/photoprism/photoprism/internal/entity"
	"github.com/photoprism/photoprism/internal/event"
	"github.com/photoprism/photoprism/pkg/clean"
)

var log = event.Log

// ApplyResult summarizes a cull apply pass.
type ApplyResult struct {
	GroupsCreated  int
	PhotosArchived int
	PhotosSkipped  int
}

// Apply persists cull groups and optionally soft-archives reject members.
func Apply(groups []Group, opt Options) (ApplyResult, error) {
	var result ApplyResult

	if opt.MinSize < 2 {
		opt.MinSize = 2
	}

	for _, g := range groups {
		if len(g.Members) < opt.MinSize {
			result.PhotosSkipped += len(g.Members)
			continue
		}

		uids := make([]string, 0, len(g.Members))
		for _, m := range g.Members {
			uids = append(uids, m.PhotoUID)
		}

		existing, err := entity.PhotoUIDsInCulls(uids)
		if err != nil {
			return result, err
		}

		if !opt.Force {
			blocked := false
			for _, uid := range uids {
				if _, ok := existing[uid]; ok {
					blocked = true
					break
				}
			}
			if blocked {
				result.PhotosSkipped += len(g.Members)
				continue
			}
		} else {
			for uid, cullUID := range existing {
				if c := entity.FindCullByUID(cullUID); c != nil && c.CullSrc == entity.CullSrcAuto && c.ReviewedAt == nil {
					_ = c.Delete()
				} else if cullUID != "" {
					_ = entity.DeletePhotoCull(uid, cullUID)
				}
			}
		}

		cullEntity := entity.NewCull(g.KeeperUID, entity.CullSrcAuto)
		cullEntity.MemberCount = len(g.Members)
		cullEntity.CullScore = g.Score

		if err := cullEntity.Create(); err != nil {
			return result, fmt.Errorf("create cull: %w", err)
		}

		for _, m := range g.Members {
			role := m.Role
			if role == "" {
				if m.PhotoUID == g.KeeperUID {
					role = entity.CullRoleKeeper
				} else {
					role = entity.CullRoleReject
				}
			}

			pc := entity.NewPhotoCull(m.PhotoUID, cullEntity.CullUID, role)
			pc.RankScore = m.RankScore
			pc.FileDiff = m.FileDiff

			archived := false
			if opt.AutoArchive && role == entity.CullRoleReject {
				photo := entity.FindPhoto(entity.Photo{PhotoUID: m.PhotoUID})
				if photo == nil {
					result.PhotosSkipped++
				} else if opt.SkipFavorite && photo.PhotoFavorite {
					pc.MemberRole = entity.CullRoleKeep
				} else if photo.DeletedAt != nil {
					// Already archived.
				} else if photo.PhotoQuality < 0 {
					// Hidden / invalid.
				} else if err := photo.Archive(); err != nil {
					log.Errorf("cull: failed to archive %s (%s)", clean.Log(m.PhotoUID), err)
				} else {
					archived = true
					result.PhotosArchived++
					log.Infof("cull: archived %s as reject in %s", clean.Log(m.PhotoUID), clean.Log(cullEntity.CullUID))
				}
			}

			pc.AutoArchived = archived

			if created := entity.FirstOrCreatePhotoCull(pc); created == nil {
				log.Errorf("cull: failed to add member %s to %s", clean.Log(m.PhotoUID), clean.Log(cullEntity.CullUID))
			}
		}

		result.GroupsCreated++
		log.Infof("cull: created group %s with %d members (keeper %s)", clean.Log(cullEntity.CullUID), cullEntity.MemberCount, clean.Log(cullEntity.KeeperUID))
	}

	return result, nil
}

// RestoreGroup restores all auto-archived rejects in a cull group.
func RestoreGroup(cullUID string) error {
	cullEntity := entity.FindCullByUID(cullUID)
	if cullEntity == nil {
		return fmt.Errorf("cull not found")
	}

	members, err := cullEntity.Members()
	if err != nil {
		return err
	}

	now := entity.Now()

	for _, m := range members {
		if !m.AutoArchived && m.MemberRole != entity.CullRoleReject {
			continue
		}

		photo := entity.FindPhoto(entity.Photo{PhotoUID: m.PhotoUID})
		if photo == nil {
			continue
		}

		if err := photo.Restore(); err != nil {
			return err
		}

		_ = m.Updates(entity.Values{"auto_archived": false, "updated_at": now})
	}

	return cullEntity.Updates(entity.Values{"reviewed_at": now})
}

// Dissolve removes a cull group without deleting photo files.
func Dissolve(cullUID string) error {
	cullEntity := entity.FindCullByUID(cullUID)
	if cullEntity == nil {
		return fmt.Errorf("cull not found")
	}

	return cullEntity.Delete()
}

// LoadCandidates loads cull candidates from the index.
// When since is zero, all eligible photos are considered.
func LoadCandidates(limit int, since time.Time) ([]Candidate, error) {
	type row struct {
		PhotoUID        string
		TakenAt         time.Time
		CameraID        uint
		PhotoQuality    int
		PhotoResolution int
		PhotoFavorite   bool
		FileDiff        int
	}

	db := entity.Db().Table("photos").
		Select(`photos.photo_uid AS photo_uid,
			photos.taken_at AS taken_at,
			photos.camera_id AS camera_id,
			photos.photo_quality AS photo_quality,
			photos.photo_resolution AS photo_resolution,
			photos.photo_favorite AS photo_favorite,
			files.file_diff AS file_diff`).
		Joins(`JOIN files ON files.photo_id = photos.id AND files.file_primary = 1 AND files.file_missing = 0 AND files.deleted_at IS NULL`).
		Where("photos.deleted_at IS NULL").
		Where("photos.photo_quality > -1").
		Where("photos.photo_type = ?", entity.MediaImage).
		Where("files.file_diff > 0").
		Order("photos.taken_at ASC, photos.photo_uid ASC")

	if !since.IsZero() {
		db = db.Where("photos.taken_at >= ?", since)
	}

	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []row
	if err := db.Scan(&rows).Error; err != nil {
		return nil, err
	}

	uids := make([]string, 0, len(rows))
	for _, r := range rows {
		uids = append(uids, r.PhotoUID)
	}

	inCull, err := entity.PhotoUIDsInCulls(uids)
	if err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, len(rows))
	for _, r := range rows {
		_, already := inCull[r.PhotoUID]
		out = append(out, Candidate{
			PhotoUID:    r.PhotoUID,
			TakenAt:     r.TakenAt,
			CameraID:    r.CameraID,
			FileDiff:    r.FileDiff,
			Quality:     r.PhotoQuality,
			Resolution:  r.PhotoResolution,
			Favorite:    r.PhotoFavorite,
			AlreadyCull: already,
		})
	}

	return out, nil
}
