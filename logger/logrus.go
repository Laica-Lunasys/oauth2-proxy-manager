package logger

import (
	"os"

	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

// Init - InitLogging
func Init() {
	logrus.SetFormatter(&prefixed.TextFormatter{
		TimestampFormat: "2006/01/02 15:04:05",
		FullTimestamp:   true,
	})
	logrus.SetOutput(os.Stdout)
	if len(os.Getenv("DEBUG")) != 0 {
		logrus.SetLevel(logrus.DebugLevel)
	}
}
