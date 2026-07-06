package utils

import (
	"io"
	"log"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// SetupLogging directs the standard logger to both stdout and a size/count-bounded
// rotating file (per config.LogFilePath/LogMaxSizeMB/LogMaxBackups), so logs stay
// visible in the console while also persisting to disk. The returned io.Closer
// releases the log file's handle; callers that need the file removable (e.g. tests
// cleaning up a temp directory) should Close() it once done.
func SetupLogging(config *Config) io.Closer {
	rotator := &lumberjack.Logger{
		Filename:   config.LogFilePath,
		MaxSize:    config.LogMaxSizeMB,
		MaxBackups: config.LogMaxBackups,
	}

	log.SetOutput(io.MultiWriter(os.Stdout, rotator))

	return rotator
}
