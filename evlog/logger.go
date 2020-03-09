package evlog

import "github.com/sirupsen/logrus"

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

var logger = NewNoneLogger()

func SetLogger(l Logger) {
	logger = l
}

func Debug(args ...interface{}) {
	logger.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func Info(args ...interface{}) {
	logger.Info(args...)
}

func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Warning(args ...interface{}) {
	logger.Warning(args...)
}

func Warningf(format string, args ...interface{}) {
	logger.Warningf(format, args...)
}

func Error(args ...interface{}) {
	logger.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}

func NewDebugLogger() Logger {
	l := &stdLogger{logrus.New()}
	l.logger.SetLevel(logrus.DebugLevel)
	return l
}

func NewLogger() Logger {
	l := &stdLogger{logrus.New()}
	return l
}

type stdLogger struct {
	logger *logrus.Logger
}

func (l *stdLogger) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

func (l *stdLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *stdLogger) Info(args ...interface{}) {
	l.logger.Info(args...)
}

func (l *stdLogger) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *stdLogger) Warning(args ...interface{}) {
	l.logger.Warning(args...)
}

func (l *stdLogger) Warningf(format string, args ...interface{}) {
	l.logger.Warningf(format, args...)
}

func (l *stdLogger) Error(args ...interface{}) {
	l.logger.Error(args...)
}

func (l *stdLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

func (l *stdLogger) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

func (l *stdLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}

func NewNoneLogger() Logger {
	return &noneLogger{}
}

type noneLogger struct{}

func (l *noneLogger) Debug(args ...interface{}) {}

func (l *noneLogger) Debugf(format string, args ...interface{}) {}

func (l *noneLogger) Info(args ...interface{}) {}

func (l *noneLogger) Infof(format string, args ...interface{}) {}

func (l *noneLogger) Warning(args ...interface{}) {}

func (l *noneLogger) Warningf(format string, args ...interface{}) {}

func (l *noneLogger) Error(args ...interface{}) {}

func (l *noneLogger) Errorf(format string, args ...interface{}) {}

func (l *noneLogger) Fatal(args ...interface{}) {}

func (l *noneLogger) Fatalf(format string, args ...interface{}) {}
