package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Link struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ShortCode   string     `gorm:"size:32;uniqueIndex;not null" json:"short_code"`
	OriginalURL string     `gorm:"type:text;not null" json:"original_url"`
	UserID      *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	User        *User      `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"user,omitempty"`
	IsActive    bool       `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at,omitempty"`
}

func (Link) TableName() string {
	return "links"
}

// BeforeCreate hook to auto-generate UUIDv7 if not provided
func (l *Link) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		l.ID = id
	}
	return nil
}

type ClickEvent struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id,omitempty"`
	ShortCode   string    `gorm:"type:varchar(32);index;not null" json:"short_code"`
	ClickedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"clicked_at"`
	IPHash      string    `gorm:"type:varchar(64)" json:"ip_hash"`
	CountryCode string    `gorm:"type:varchar(10)" json:"country_code"`
	City        string    `gorm:"type:varchar(100)" json:"city"`
	DeviceType  string    `gorm:"type:varchar(50)" json:"device_type"`
	Browser     string    `gorm:"type:varchar(50)" json:"browser"`
	OS          string    `gorm:"type:varchar(50)" json:"os"`
	Referer     string    `gorm:"type:text" json:"referer"`
}

func (ClickEvent) TableName() string {
	return "click_events"
}
