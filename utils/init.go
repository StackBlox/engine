package utils

import (
	log "github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
)

func init() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	initLogger()
	initDb()
	initMinio()
	initDockerClient()
}
