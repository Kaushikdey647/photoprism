package entity

import (
	"fmt"
	"time"
)

// PhotoCulls is a helper alias for collections of PhotoCull relations.
type PhotoCulls []PhotoCull

// PhotoCull represents membership of a photo in a cull group.
type PhotoCull struct {
	PhotoUID     string    `gorm:"type:VARBINARY(42);primary_key;auto_increment:false" json:"PhotoUID" yaml:"UID"`
	CullUID      string    `gorm:"type:VARBINARY(42);primary_key;auto_increment:false;index" json:"CullUID" yaml:"-"`
	MemberRole   string    `gorm:"type:VARBINARY(8);default:'reject';" json:"MemberRole" yaml:"MemberRole,omitempty"`
	RankScore    float64   `json:"RankScore" yaml:"RankScore,omitempty"`
	FileDiff     int       `json:"FileDiff" yaml:"FileDiff,omitempty"`
	AutoArchived bool      `json:"AutoArchived" yaml:"AutoArchived,omitempty"`
	CreatedAt    time.Time `json:"CreatedAt" yaml:"CreatedAt,omitempty"`
	UpdatedAt    time.Time `json:"UpdatedAt" yaml:"-"`
	Photo        *Photo    `gorm:"PRELOAD:false" yaml:"-"`
	Cull         *Cull     `gorm:"PRELOAD:false" yaml:"-"`
}

// TableName returns the entity table name.
func (PhotoCull) TableName() string {
	return "photos_culls"
}

// NewPhotoCull creates a new photo-to-cull relation.
func NewPhotoCull(photoUID, cullUID, role string) *PhotoCull {
	if role == "" {
		role = CullRoleReject
	}

	return &PhotoCull{
		PhotoUID:   photoUID,
		CullUID:    cullUID,
		MemberRole: role,
	}
}

// Create inserts a new membership row.
func (m *PhotoCull) Create() error {
	return Db().Create(m).Error
}

// Save updates an existing membership or inserts a new one.
func (m *PhotoCull) Save() error {
	return Db().Save(m).Error
}

// Updates updates selected columns for this membership.
func (m *PhotoCull) Updates(values interface{}) error {
	return UnscopedDb().Model(m).Updates(values).Error
}

// FirstOrCreatePhotoCull returns the persisted membership, creating it when necessary.
func FirstOrCreatePhotoCull(m *PhotoCull) *PhotoCull {
	result := PhotoCull{}

	if err := Db().Where("photo_uid = ? AND cull_uid = ?", m.PhotoUID, m.CullUID).First(&result).Error; err == nil {
		return &result
	} else if err := m.Create(); err != nil {
		log.Errorf("photo-cull: %s", err)
		return nil
	}

	return m
}

// FindPhotoCull returns a membership by photo and cull UID.
func FindPhotoCull(photoUID, cullUID string) *PhotoCull {
	if photoUID == "" || cullUID == "" {
		return nil
	}

	result := PhotoCull{}

	if err := Db().Where("photo_uid = ? AND cull_uid = ?", photoUID, cullUID).First(&result).Error; err != nil {
		return nil
	}

	return &result
}

// FindPhotoCullByPhoto returns the cull membership for a photo, if any.
func FindPhotoCullByPhoto(photoUID string) *PhotoCull {
	if photoUID == "" {
		return nil
	}

	result := PhotoCull{}

	if err := Db().Where("photo_uid = ?", photoUID).First(&result).Error; err != nil {
		return nil
	}

	return &result
}

// PhotoUIDsInCulls returns the set of photo UIDs that already belong to a cull group.
func PhotoUIDsInCulls(uids []string) (map[string]string, error) {
	result := make(map[string]string)

	if len(uids) == 0 {
		return result, nil
	}

	var rows PhotoCulls

	if err := Db().Where("photo_uid IN (?)", uids).Find(&rows).Error; err != nil {
		return result, err
	}

	for _, row := range rows {
		result[row.PhotoUID] = row.CullUID
	}

	return result, nil
}

// DeletePhotoCull removes a membership row.
func DeletePhotoCull(photoUID, cullUID string) error {
	if photoUID == "" || cullUID == "" {
		return fmt.Errorf("photo and cull uid required")
	}

	return Db().Where("photo_uid = ? AND cull_uid = ?", photoUID, cullUID).Delete(&PhotoCull{}).Error
}
