package config

import (
	"os"

	"github.com/rs/zerolog"
)

var (
	logger            zerolog.Logger
	consolePrintJSON  bool
	loggerInitialized bool
)

func RunningInTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == os.ModeCharDevice
}

func DiscardLogEvents(ev ...*zerolog.Event) {
	for _, e := range ev {
		e.Discard()
	}
}

func LogString(key string, val string, ev ...*zerolog.Event) {
	for _, e := range ev {
		e.Str(key, val)
	}
}

func LogInt(key string, val int, ev ...*zerolog.Event) {
	for _, e := range ev {
		e.Int(key, val)
	}
}

func InitLogger(printJSON bool) error {
	consolePrintJSON = printJSON || !RunningInTerminal()
	var err error
	loggerInitialized = false // resets the logger
	logger, err = GetLogger()
	if err == nil {
		loggerInitialized = true
	}
	return err
}

// GetLogger initializes and returns a zerolog.Logger instance. If an error occurs, it returns the default logger.
func GetLogger() (zerolog.Logger, error) {
	if loggerInitialized {
		return logger, nil
	}

	logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	var consoleOut zerolog.Logger
	var logFileOut zerolog.Logger
	if !consolePrintJSON && RunningInTerminal() {
		consoleOut = zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	} else {
		consoleOut = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	cfg, err := GetConfig()
	if err != nil {
		return logger, err
	}

	if cfg.LogFile != "" {
		logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
			return logger, err
		}
		logFileOut = zerolog.New(logFile)
		logger = zerolog.New(zerolog.MultiLevelWriter(consoleOut, logFileOut))
	} else {
		logger = consoleOut
	}
	loggerInitialized = true
	return logger, nil
}
