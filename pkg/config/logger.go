package config

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"golang.org/x/term"
)

var (
	logger            zerolog.Logger
	consolePrintJSON  bool
	loggerInitialized bool
	runningInTerminal = term.IsTerminal(int(os.Stdin.Fd()))
	logFile           *os.File
)

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
	consolePrintJSON = printJSON || !runningInTerminal
	loggerInitialized = false // resets the logger
	var err error
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

	cfg, err := GetConfig()
	if err != nil {
		return logger, err
	}

	var writer zerolog.LevelWriter
	var writers []io.Writer

	if cfg.PrintLogToConsole {
		if consolePrintJSON {
			writers = append(writers, os.Stdout)
		} else {
			writers = append(writers, zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
				w.NoColor = !runningInTerminal
			}))
		}
	}
	if cfg.LogFile != "" {
		logFile, err = os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return logger, err
		}
		writers = append(writers, logFile)
	}

	if len(writers) != 0 {
		writer = zerolog.MultiLevelWriter(writers...)
		logger = zerolog.New(writer).With().Timestamp().Logger()
	} else {
		// If no file is specified and console logging is disabled, no logging will be done
		logger = zerolog.Nop()
	}
	loggerInitialized = true
	return logger, nil
}

func SetLogger(l zerolog.Logger) {
	logger = l
	loggerInitialized = true
}
