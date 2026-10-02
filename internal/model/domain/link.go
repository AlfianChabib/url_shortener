package domain

import (
	"time"

	"github.com/google/uuid"
)

type Link struct {
	ID          int64      `gorm:"primaryKey;autoIncrement:false" json:"id"`
	ShortCode   string     `gorm:"size:32;uniqueIndex;not null" json:"short_code"`
	OriginalURL string     `gorm:"type:text;not null" json:"original_url"`
	UserID      *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	IsActive    bool       `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at,omitempty"`
}

func (Link) TableName() string {
	return "links"
}

type ClickEvent struct {
	ShortCode   string    `gorm:"type:varchar(32);index;not null" json:"short_code"`
	ClickedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"clicked_at"`
	IPHash      string    `gorm:"type:char(32)" json:"ip_hash"`
	CountryCode string    `gorm:"type:char(2)" json:"country_code"`
	City        string    `gorm:"type:varchar(100)" json:"city"`
	DeviceType  string    `gorm:"type:varchar(50)" json:"device_type"`
	Browser     string    `gorm:"type:varchar(50)" json:"browser"`
	OS          string    `gorm:"type:varchar(50)" json:"os"`
	Referer     string    `gorm:"type:text" json:"referer"`
}

func (ClickEvent) TableName() string {
	return "click_events"
}
