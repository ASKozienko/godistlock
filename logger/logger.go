package logger

type Logger interface {
	Debug(msg string, args ...interface{})
	Error(msg string, arts ...interface{})
}
