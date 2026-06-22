package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	return []byte(j), nil
}

func (j *JSON) Scan(value any) error {
	data, ok := value.([]byte)
	if !ok {
		if text, stringOK := value.(string); stringOK {
			data = []byte(text)
		} else {
			return fmt.Errorf("cannot scan JSON from %T", value)
		}
	}
	*j = append((*j)[:0], data...)
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	return j, nil
}

type Note struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index:idx_notes_user_updated" json:"-"`
	Title      string         `gorm:"type:varchar(500);not null;default:'Untitled'" json:"title"`
	Content    JSON           `gorm:"type:jsonb;not null;default:'[]'" json:"content"`
	Icon       string         `gorm:"type:varchar(32);not null;default:'📝'" json:"icon"`
	IsFavorite bool           `gorm:"not null;default:false" json:"is_favorite"`
	IsArchived bool           `gorm:"not null;default:false;index" json:"is_archived"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `gorm:"index:idx_notes_user_updated,sort:desc" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (note *Note) BeforeCreate(_ *gorm.DB) error {
	if note.ID == uuid.Nil {
		note.ID = uuid.New()
	}
	return nil
}
