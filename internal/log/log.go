package log

import (
	"os"
	"sync"

	"github.com/charmbracelet/lipgloss"
	cblog "github.com/charmbracelet/log"
)

// Logger embeds the Charm Logger and adds Printf/Fatalf
type Logger struct{ *cblog.Logger }

// Printf routes goose/info-style logs through Infof.
func (l *Logger) Printf(format string, v ...any) { l.Infof(format, v...) }

// Fatalf keeps goose’s contract of exiting the program.
func (l *Logger) Fatalf(format string, v ...any) { l.Logger.Fatalf(format, v...) }

var (
	logger     *Logger
	initLogger sync.Once
)

// GetLogger returns a logger instance
func GetLogger() *Logger {
	initLogger.Do(func() {
		styles := cblog.DefaultStyles()
		// Attempt to match the colors used by qml/quickshell logs
		styles.Levels[cblog.FatalLevel] = lipgloss.NewStyle().
			SetString(" FATAL").
			Foreground(lipgloss.Color("1"))
		styles.Levels[cblog.ErrorLevel] = lipgloss.NewStyle().
			SetString(" ERROR").
			Foreground(lipgloss.Color("9"))
		styles.Levels[cblog.WarnLevel] = lipgloss.NewStyle().
			SetString("  WARN").
			Foreground(lipgloss.Color("3"))
		styles.Levels[cblog.InfoLevel] = lipgloss.NewStyle().
			SetString("  INFO").
			Foreground(lipgloss.Color("2"))
		styles.Levels[cblog.DebugLevel] = lipgloss.NewStyle().
			SetString(" DEBUG").
			Foreground(lipgloss.Color("4"))

		base := cblog.New(os.Stderr)
		base.SetStyles(styles)
		base.SetReportTimestamp(false)
		if level := os.Getenv("DANKSEARCH_DEBUG"); level != "" {
			base.SetLevel(cblog.DebugLevel)
		} else {
			base.SetLevel(cblog.InfoLevel)
		}

		logger = &Logger{base}
	})
	return logger
}

// * Convenience wrappers

func Debug(msg any, keyvals ...any)  { GetLogger().Debug(msg, keyvals...) }
func Debugf(format string, v ...any) { GetLogger().Debugf(format, v...) }
func Info(msg any, keyvals ...any)   { GetLogger().Info(msg, keyvals...) }
func Infof(format string, v ...any)  { GetLogger().Infof(format, v...) }
func Warn(msg any, keyvals ...any)   { GetLogger().Warn(msg, keyvals...) }
func Warnf(format string, v ...any)  { GetLogger().Warnf(format, v...) }
func Error(msg any, keyvals ...any)  { GetLogger().Error(msg, keyvals...) }
func Errorf(format string, v ...any) { GetLogger().Errorf(format, v...) }
func Fatal(msg any, keyvals ...any)  { GetLogger().Fatal(msg, keyvals...) }
func Fatalf(format string, v ...any) { GetLogger().Logger.Fatalf(format, v...) }
