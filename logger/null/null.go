package null

type Logger struct {
}

func New() *Logger {
	return &Logger{}
}

func (l Logger) Debug(msg string, args ...interface{}) {

}

func (l Logger) Error(msg string, arts ...interface{}) {

}
