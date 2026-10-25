package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/middleware"
	"github.com/mhandyalf/handy-notion/internal/model"
	"github.com/mhandyalf/handy-notion/internal/repository"
	"gorm.io/gorm"
)

type noteRepository interface {
	List(uuid.UUID, bool, string) ([]model.Note, error)
	Get(uuid.UUID, uuid.UUID) (*model.Note, error)
	Create(*model.Note) error
	Update(uuid.UUID, uuid.UUID, time.Time, map[string]any) (*model.Note, error)
	Delete(uuid.UUID, uuid.UUID, time.Time) error
}

type NoteHandler struct{ notes noteRepository }

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
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}
	var input struct {
		noteInput
		ExpectedUpdatedAt json.RawMessage `json:"expected_updated_at"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note"})
		return
	}
	var token string
	if len(input.ExpectedUpdatedAt) > 0 {
		if err := json.Unmarshal(input.ExpectedUpdatedAt, &token); err != nil || string(input.ExpectedUpdatedAt) == "null" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expected_updated_at"})
			return
		}
	}
	expected, ok := expectedTimestamp(c, token, len(input.ExpectedUpdatedAt) > 0)
	if !ok {
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
	note, err := h.notes.Update(userID(c), id, expected, updates)
	if errors.Is(err, repository.ErrNoteConflict) {
		noteConflict(c)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save note"})
		return
	}
	c.JSON(http.StatusOK, note)
}

func (h *NoteHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}
	query := c.Request.URL.Query()
	token := query.Get("expected_updated_at")
	_, present := query["expected_updated_at"]
	expected, ok := expectedTimestamp(c, token, present)
	if !ok {
		return
	}
	err = h.notes.Delete(userID(c), id, expected)
	if errors.Is(err, repository.ErrNoteConflict) {
		noteConflict(c)
		return
	}
	if err != nil {
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

func noteConflict(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{"error": "note has changed; reload before retrying", "code": "note_conflict"})
}

func expectedTimestamp(c *gin.Context, token string, present bool) (time.Time, bool) {
	if !present {
		c.JSON(http.StatusPreconditionRequired, gin.H{"error": "expected_updated_at is required"})
		return time.Time{}, false
	}
	expected, err := time.Parse(time.RFC3339Nano, token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected_updated_at must be an RFC3339Nano timestamp"})
		return time.Time{}, false
	}
	return expected, true
}

func validBlocks(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return json.Valid(trimmed) && len(trimmed) >= 2 && trimmed[0] == '['
}
