package main

import (
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/mhandyalf/handy-notion/internal/app"
)

func main() {
	router, err := app.NewRouterFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
