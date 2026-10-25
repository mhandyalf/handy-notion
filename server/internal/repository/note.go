package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NoteRepository struct{ db *gorm.DB }

func NewNoteRepository(db *gorm.DB) *NoteRepository { return &NoteRepository{db: db} }

func (r *NoteRepository) List(userID uuid.UUID, archived bool, query string) ([]model.Note, error) {
	var notes []model.Note
	db := r.db.Where("user_id = ? AND is_archived = ?", userID, archived)
	if query = strings.TrimSpace(query); query != "" {
		like := "%" + query + "%"
		db = db.Where("title ILIKE ? OR content::text ILIKE ?", like, like)
	}
	err := db.Order("is_favorite DESC, updated_at DESC").Find(&notes).Error
	return notes, err
}

func (r *NoteRepository) Get(userID, id uuid.UUID) (*model.Note, error) {
	var note model.Note
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&note).Error
	return &note, err
}

// Read back database-normalized timestamp precision for the first conditional save.
func (r *NoteRepository) Create(note *model.Note) error {
	return r.db.Clauses(clause.Returning{}).Create(note).Error
}

// ErrNoteConflict also covers missing, deleted, and unowned notes, without
// revealing whether another user's note exists.
var ErrNoteConflict = errors.New("note conflict")

func (r *NoteRepository) Update(userID, id uuid.UUID, expected time.Time, values map[string]any) (*model.Note, error) {
	// PostgreSQL stores microseconds. Never let driver truncation turn a distinct
	// nanosecond token into a matching version.
	if expected.Nanosecond()%1000 != 0 {
		return nil, ErrNoteConflict
	}
	updates := make(map[string]any, len(values)+1)
	for key, value := range values {
		updates[key] = value
	}
	// Compute this in the database against the matched row, not the application
	// clock. Explicit truncation prevents rounding from defeating monotonicity.
	updates["updated_at"] = gorm.Expr("GREATEST(date_trunc('microseconds', clock_timestamp()), updated_at + INTERVAL '1 microsecond')")
	var note model.Note
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&note).Clauses(clause.Returning{}).
			Where("id = ? AND user_id = ? AND updated_at = ?", id, userID, expected).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNoteConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *NoteRepository) Delete(userID, id uuid.UUID, expected time.Time) error {
	if expected.Nanosecond()%1000 != 0 {
		return ErrNoteConflict
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND user_id = ? AND updated_at = ?", id, userID, expected).
			Delete(&model.Note{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNoteConflict
		}
		return nil
	})
}
