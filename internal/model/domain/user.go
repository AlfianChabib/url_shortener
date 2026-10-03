package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Email        string     `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Username     string     `gorm:"size:50;uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	IsActive     bool       `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	Links        []Link     `gorm:"foreignKey:UserID" json:"links,omitempty"`
}

func (User) TableName() string {
	return "users"
}

// BeforeCreate hook to auto-generate UUIDv7 if not provided
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		u.ID = id
	}
	return nil
}

