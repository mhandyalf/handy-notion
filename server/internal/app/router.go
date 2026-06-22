package app

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/mhandyalf/handy-notion/internal/handler"
	"github.com/mhandyalf/handy-notion/internal/middleware"
	"github.com/mhandyalf/handy-notion/internal/model"
	"github.com/mhandyalf/handy-notion/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewRouterFromEnv() (*gin.Engine, error) {
	dsn := os.Getenv("DB_POSTGRES")
	if dsn == "" {
		return nil, fmt.Errorf("DB_POSTGRES is required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := db.AutoMigrate(&model.Note{}); err != nil {
		return nil, fmt.Errorf("migrate notes: %w", err)
	}
	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:8081"
	}
	return NewRouter(db, authURL, allowedOrigins()), nil
}

func NewRouter(db *gorm.DB, authURL string, origins []string) *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{AllowOrigins: origins, AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Origin", "Content-Type", "Authorization", "Accept"}, MaxAge: 12 * time.Hour}))
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	notes := handler.NewNoteHandler(repository.NewNoteRepository(db))
	api := r.Group("/api", middleware.HandyAuth(authURL, nil))
	api.GET("/notes", notes.List)
	api.POST("/notes", notes.Create)
	api.GET("/notes/:id", notes.Get)
	api.PUT("/notes/:id", notes.Update)
	api.DELETE("/notes/:id", notes.Delete)
	return r
}

func allowedOrigins() []string {
	value := os.Getenv("CORS_ORIGINS")
	if value == "" {
		value = "http://localhost:5174,http://127.0.0.1:5174"
	}
	var origins []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			origins = append(origins, item)
		}
	}
	return origins
}
