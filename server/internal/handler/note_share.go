package handler

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mhandyalf/handy-notion/internal/repository"
	"gorm.io/gorm"
)

type NoteShareHandler struct {
	shares *repository.NoteShareRepository
}

func NewNoteShareHandler(shares *repository.NoteShareRepository) *NoteShareHandler {
	return &NoteShareHandler{shares: shares}
}

// PublicNoteHeaders also covers unknown public paths and unsupported methods.
func PublicNoteHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/api/public/notes" || strings.HasPrefix(c.Request.URL.Path, "/api/public/notes/") {
			c.Header("Cache-Control", "no-store")
			c.Header("Referrer-Policy", "no-referrer")
			c.Header("X-Robots-Tag", "noindex,nofollow,noarchive")
		}
		c.Next()
	}
}

func shareError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to access note share"})
	}
}

func shareID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		shareError(c, gorm.ErrRecordNotFound)
		return uuid.Nil, false
	}
	return id, true
}

func (h *NoteShareHandler) Status(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, ok := shareID(c)
	if !ok {
		return
	}
	token, err := h.shares.Status(userID(c), id)
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *NoteShareHandler) Enable(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, ok := shareID(c)
	if !ok {
		return
	}
	token, err := h.shares.Enable(userID(c), id)
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *NoteShareHandler) Revoke(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, ok := shareID(c)
	if !ok {
		return
	}
	if err := h.shares.Revoke(userID(c), id); err != nil {
		shareError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NoteShareHandler) Public(c *gin.Context) {
	token := c.Param("token")
	if len(token) != 64 {
		shareError(c, gorm.ErrRecordNotFound)
		return
	}
	if _, err := hex.DecodeString(token); err != nil {
		shareError(c, gorm.ErrRecordNotFound)
		return
	}
	note, err := h.shares.Public(token)
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, note)
}
