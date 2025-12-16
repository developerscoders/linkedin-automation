package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type Logger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Warn(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
	Fatal(msg string, keyvals ...interface{})
}

type zeroLogger struct {
	logger zerolog.Logger
}

func New(level string, format string) Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	var output io.Writer = os.Stdout
	if format == "text" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	l, err := zerolog.ParseLevel(level)
	if err != nil {
		l = zerolog.InfoLevel
	}

	z := zerolog.New(output).Level(l).With().Timestamp().Logger()

	return &zeroLogger{logger: z}
}

func (l *zeroLogger) Debug(msg string, keyvals ...interface{}) {
	l.log(l.logger.Debug(), msg, keyvals...)
}

func (l *zeroLogger) Info(msg string, keyvals ...interface{}) {
	l.log(l.logger.Info(), msg, keyvals...)
}

func (l *zeroLogger) Warn(msg string, keyvals ...interface{}) {
	l.log(l.logger.Warn(), msg, keyvals...)
}

func (l *zeroLogger) Error(msg string, keyvals ...interface{}) {
	l.log(l.logger.Error(), msg, keyvals...)
}

func (l *zeroLogger) Fatal(msg string, keyvals ...interface{}) {
	l.log(l.logger.Fatal(), msg, keyvals...)
}

func (l *zeroLogger) log(e *zerolog.Event, msg string, keyvals ...interface{}) {
	if e == nil {
		return
	}

	// Add fields
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key, ok := keyvals[i].(string)
			if ok {
				e.Interface(key, keyvals[i+1])
			}
		}
	}

	e.Msg(msg)
}
