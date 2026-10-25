package model

import (
	"time"

	"github.com/google/uuid"
)

// NoteShare is deliberately separate from Note: normal note reads and autosaves
// must never expose or overwrite the bearer token. Deleting a share revokes it.
type NoteShare struct {
	NoteID uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Token  string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`
}

// PublicNote is the complete allowlist for anonymous note responses.
type PublicNote struct {
	Title     string    `json:"title"`
	Icon      string    `json:"icon"`
	Content   JSON      `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}
