package repository

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NoteShareRepository struct{ db *gorm.DB }

func NewNoteShareRepository(db *gorm.DB) *NoteShareRepository { return &NoteShareRepository{db: db} }

func (r *NoteShareRepository) Status(userID, id uuid.UUID) (*string, error) {
	// One statement checks ownership and reads the share from the same snapshot.
	var row struct{ Token *string }
	result := r.db.Model(&model.Note{}).Select("note_shares.token").
		Joins("LEFT JOIN note_shares ON note_shares.note_id = notes.id").
		Where("notes.id = ? AND notes.user_id = ?", id, userID).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return row.Token, nil
}

func (r *NoteShareRepository) Enable(userID, id uuid.UUID) (string, error) {
	var token string
	err := r.withOwnedNote(userID, id, func(tx *gorm.DB) error {
		var share model.NoteShare
		err := tx.Where("note_id = ?", id).First(&share).Error
		if err == nil {
			token = share.Token
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return err
		}
		token = hex.EncodeToString(random[:])
		return tx.Create(&model.NoteShare{NoteID: id, Token: token}).Error
	})
	return token, err
}

func (r *NoteShareRepository) Revoke(userID, id uuid.UUID) error {
	return r.withOwnedNote(userID, id, func(tx *gorm.DB) error {
		return tx.Where("note_id = ?", id).Delete(&model.NoteShare{}).Error
	})
}

func (r *NoteShareRepository) withOwnedNote(userID, id uuid.UUID, fn func(*gorm.DB) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var note model.Note
		if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).First(&note).Error; err != nil {
			return err
		}
		return fn(tx)
	})
}

func (r *NoteShareRepository) Public(token string) (*model.PublicNote, error) {
	var note model.PublicNote
	result := r.db.Model(&model.Note{}).
		Select("notes.title, notes.icon, notes.content, notes.updated_at").
		Joins("JOIN note_shares ON note_shares.note_id = notes.id").
		Where("note_shares.token = ? AND notes.is_archived = ?", token, false).Scan(&note)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &note, nil
}
