package logger

import (
	"go.uber.org/zap"

	"github.com/mariana-kep/yourtaskplanner/internal/config"
)

type Logger struct {
	*zap.SugaredLogger
}

func New(cfg *config.Config) *Logger {
	l, _ := zap.NewDevelopment()
	s := l.Sugar()
	return &Logger{s}
}
