package entity

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"

	"github.com/photoprism/photoprism/pkg/rnd"
)

const (
	// CullUID is the UID prefix for cull groups.
	CullUID = byte('n')

	// Cull member roles.
	CullRoleKeeper = "keeper"
	CullRoleReject = "reject"
	CullRoleKeep   = "keep"

	// Cull sources.
	CullSrcAuto   = "auto"
	CullSrcManual = "manual"
)

// Culls is a helper slice type for cull groups.
type Culls []Cull

// Cull represents a near-duplicate / burst cull group over multiple photos.
type Cull struct {
	ID          uint       `gorm:"primary_key" json:"ID" yaml:"-"`
	CullUID     string     `gorm:"type:VARBINARY(42);unique_index;" json:"UID" yaml:"UID"`
	KeeperUID   string     `gorm:"type:VARBINARY(42);index;" json:"KeeperUID" yaml:"KeeperUID"`
	MemberCount int        `json:"MemberCount" yaml:"MemberCount"`
	CullScore   float64    `json:"CullScore" yaml:"CullScore,omitempty"`
	CullSrc     string     `gorm:"type:VARBINARY(8);default:'auto';" json:"CullSrc" yaml:"CullSrc,omitempty"`
	ReviewedAt  *time.Time `json:"ReviewedAt,omitempty" yaml:"ReviewedAt,omitempty"`
	CreatedAt   time.Time  `json:"CreatedAt" yaml:"CreatedAt,omitempty"`
	UpdatedAt   time.Time  `json:"UpdatedAt" yaml:"UpdatedAt,omitempty"`
	Thumb       string     `gorm:"-" json:"Thumb,omitempty" yaml:"-"`
	Photos      PhotoCulls `gorm:"foreignkey:CullUID;association_foreignkey:CullUID;" json:"-" yaml:"-"`
}

// TableName returns the entity table name.
func (Cull) TableName() string {
	return "culls"
}

// NewCull creates a new cull group with a generated UID.
func NewCull(keeperUID, src string) *Cull {
	if src == "" {
		src = CullSrcAuto
	}

	return &Cull{
		CullUID:   rnd.GenerateUID(CullUID),
		KeeperUID: keeperUID,
		CullSrc:   src,
	}
}

// Create inserts a new cull group.
func (m *Cull) Create() error {
	if m.CullUID == "" {
		m.CullUID = rnd.GenerateUID(CullUID)
	}

	return Db().Create(m).Error
}

// Save updates an existing cull group or inserts a new one.
func (m *Cull) Save() error {
	if m.CullUID == "" {
		m.CullUID = rnd.GenerateUID(CullUID)
	}

	return Db().Save(m).Error
}

// Updates updates selected columns for this cull group.
func (m *Cull) Updates(values interface{}) error {
	return UnscopedDb().Model(m).Updates(values).Error
}

// Delete removes the cull group and its membership rows.
func (m *Cull) Delete() error {
	if m.CullUID == "" {
		return fmt.Errorf("cull uid must not be empty")
	}

	if err := Db().Where("cull_uid = ?", m.CullUID).Delete(&PhotoCull{}).Error; err != nil {
		return err
	}

	return Db().Delete(m).Error
}

// FindCullByUID returns a cull group by UID.
func FindCullByUID(uid string) *Cull {
	if !rnd.IsUID(uid, CullUID) {
		return nil
	}

	result := Cull{}

	if err := Db().Where("cull_uid = ?", uid).First(&result).Error; err != nil {
		return nil
	}

	return &result
}

// FirstOrCreateCull returns the persisted cull or creates it.
func FirstOrCreateCull(m *Cull) *Cull {
	if m == nil {
		return nil
	}

	if m.CullUID != "" {
		if found := FindCullByUID(m.CullUID); found != nil {
			return found
		}
	}

	if err := m.Create(); err != nil {
		log.Errorf("cull: %s", err)
		return nil
	}

	return m
}

// Members returns membership rows for this cull group.
func (m *Cull) Members() (PhotoCulls, error) {
	var result PhotoCulls

	if m.CullUID == "" {
		return result, fmt.Errorf("cull uid must not be empty")
	}

	err := Db().Where("cull_uid = ?", m.CullUID).Order("rank_score DESC, photo_uid ASC").Find(&result).Error

	return result, err
}

// PhotoUIDs returns member photo UIDs ordered by rank.
func (m *Cull) PhotoUIDs() ([]string, error) {
	members, err := m.Members()

	if err != nil {
		return nil, err
	}

	uids := make([]string, 0, len(members))

	for _, member := range members {
		uids = append(uids, member.PhotoUID)
	}

	return uids, nil
}

// SetKeeper updates the keeper photo and member roles.
func (m *Cull) SetKeeper(photoUID string) error {
	if m.CullUID == "" || photoUID == "" {
		return fmt.Errorf("cull and photo uid required")
	}

	now := Now()

	if err := Db().Model(&PhotoCull{}).Where("cull_uid = ? AND photo_uid = ?", m.CullUID, photoUID).
		Updates(Values{"member_role": CullRoleKeeper, "updated_at": now}).Error; err != nil {
		return err
	}

	if err := Db().Model(&PhotoCull{}).
		Where("cull_uid = ? AND photo_uid <> ? AND member_role = ?", m.CullUID, photoUID, CullRoleKeeper).
		Updates(Values{"member_role": CullRoleReject, "updated_at": now}).Error; err != nil {
		return err
	}

	m.KeeperUID = photoUID
	m.ReviewedAt = &now

	return m.Updates(Values{"keeper_uid": photoUID, "reviewed_at": now})
}

// ProtectMember marks a member as manually kept and restores it if archived.
func (m *Cull) ProtectMember(photoUID string) error {
	if m.CullUID == "" || photoUID == "" {
		return fmt.Errorf("cull and photo uid required")
	}

	now := Now()

	if err := Db().Model(&PhotoCull{}).Where("cull_uid = ? AND photo_uid = ?", m.CullUID, photoUID).
		Updates(Values{"member_role": CullRoleKeep, "auto_archived": false, "updated_at": now}).Error; err != nil {
		return err
	}

	photo := FindPhoto(Photo{PhotoUID: photoUID})

	if photo == nil {
		return fmt.Errorf("photo not found")
	}

	if err := photo.Restore(); err != nil {
		return err
	}

	m.ReviewedAt = &now

	return m.Updates(Values{"reviewed_at": now})
}

// AfterFind is a GORM hook placeholder for future preload helpers.
func (m *Cull) AfterFind(tx *gorm.DB) error {
	return nil
}
