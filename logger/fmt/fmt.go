package fmt

import "fmt"

type Logger struct {
}

func New() *Logger {
	return &Logger{}
}

func (l Logger) Debug(msg string, args ...interface{}) {
	fmt.Println("[DEBUG]", fmt.Sprintf(msg, args...))
}

func (l Logger) Error(msg string, args ...interface{}) {
	fmt.Println("[ERROR]", fmt.Sprintf(msg, args...))
}
