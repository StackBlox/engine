package utils

import (
	"os"

	log "github.com/sirupsen/logrus"
)

var Logger *log.Logger

func initLogger() {
	Logger = log.New()

	// Log as JSON instead of the default ASCII formatter.
	Logger.SetFormatter(&log.TextFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	Logger.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	Logger.SetLevel(log.DebugLevel)
}
