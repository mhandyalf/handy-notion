package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/middleware"
	"github.com/mhandyalf/handy-notion/internal/model"
	"github.com/mhandyalf/handy-notion/internal/repository"
	"gorm.io/gorm"
)

type NoteHandler struct{ notes *repository.NoteRepository }

type noteInput struct {
	Title      *string         `json:"title"`
	Content    json.RawMessage `json:"content"`
	Icon       *string         `json:"icon"`
	IsFavorite *bool           `json:"is_favorite"`
	IsArchived *bool           `json:"is_archived"`
}

func NewNoteHandler(notes *repository.NoteRepository) *NoteHandler { return &NoteHandler{notes: notes} }

func userID(c *gin.Context) uuid.UUID { return c.MustGet(middleware.UserIDKey).(uuid.UUID) }

func (h *NoteHandler) List(c *gin.Context) {
	archived := c.Query("archived") == "true"
	notes, err := h.notes.List(userID(c), archived, c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load notes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"notes": notes})
}

func (h *NoteHandler) Get(c *gin.Context) {
	note, ok := h.ownedNote(c)
	if ok {
		c.JSON(http.StatusOK, note)
	}
}

func (h *NoteHandler) Create(c *gin.Context) {
	var input noteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note"})
		return
	}
	note := model.Note{UserID: userID(c), Title: "Untitled", Icon: "📝", Content: model.JSON([]byte("[]"))}
	if input.Title != nil && strings.TrimSpace(*input.Title) != "" {
		note.Title = strings.TrimSpace(*input.Title)
	}
	if input.Icon != nil && strings.TrimSpace(*input.Icon) != "" {
		note.Icon = strings.TrimSpace(*input.Icon)
	}
	if len(input.Content) > 0 {
		if !validBlocks(input.Content) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content must be a JSON array"})
			return
		}
		note.Content = model.JSON(input.Content)
	}
	if err := h.notes.Create(&note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create note"})
		return
	}
	c.JSON(http.StatusCreated, note)
}

func (h *NoteHandler) Update(c *gin.Context) {
	note, ok := h.ownedNote(c)
	if !ok {
		return
	}
	var input noteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note"})
		return
	}
	updates := map[string]any{}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			title = "Untitled"
		}
		updates["title"] = title
	}
	if input.Icon != nil {
		updates["icon"] = strings.TrimSpace(*input.Icon)
	}
	if len(input.Content) > 0 {
		if !validBlocks(input.Content) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content must be a JSON array"})
			return
		}
		updates["content"] = model.JSON(input.Content)
	}
	if input.IsFavorite != nil {
		updates["is_favorite"] = *input.IsFavorite
	}
	if input.IsArchived != nil {
		updates["is_archived"] = *input.IsArchived
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	if err := h.notes.Update(note, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save note"})
		return
	}
	fresh, _ := h.notes.Get(userID(c), note.ID)
	c.JSON(http.StatusOK, fresh)
}

func (h *NoteHandler) Delete(c *gin.Context) {
	note, ok := h.ownedNote(c)
	if !ok {
		return
	}
	if err := h.notes.Delete(note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete note"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NoteHandler) ownedNote(c *gin.Context) (*model.Note, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return nil, false
	}
	note, err := h.notes.Get(userID(c), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
		return nil, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load note"})
		return nil, false
	}
	return note, true
}

func validBlocks(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return json.Valid(trimmed) && len(trimmed) >= 2 && trimmed[0] == '['
}
