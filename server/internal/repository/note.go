package repository

import (
	"strings"

	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/model"
	"gorm.io/gorm"
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

func (r *NoteRepository) Create(note *model.Note) error { return r.db.Create(note).Error }

func (r *NoteRepository) Update(note *model.Note, values map[string]any) error {
	return r.db.Model(note).Updates(values).Error
}

func (r *NoteRepository) Delete(note *model.Note) error { return r.db.Delete(note).Error }
