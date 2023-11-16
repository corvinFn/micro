package lg

import "fmt"

type Logger interface {
	Debug(...any)
	Info(...any)
	Infof(string, ...any)
	Warn(...any)
	Error(...any)
	Errorf(string, ...any)
	Panic(...any)
}

type simpleLogger struct {
}

func NewSimpleLogger() *simpleLogger {
	return &simpleLogger{}
}

func (lg *simpleLogger) Debug(v ...any) {
	fmt.Println(append(v))
}

func (lg *simpleLogger) Info(v ...any) {
	fmt.Println(append(v))
}

func (lg *simpleLogger) Infof(format string, v ...any) {
	println(fmt.Sprintf(format, v...))
}

func (lg *simpleLogger) Warn(v ...any) {
	fmt.Println(append(v))
}

func (lg *simpleLogger) Error(v ...any) {
	fmt.Println(append(v))
}

func (lg *simpleLogger) Errorf(format string, v ...any) {
	println(fmt.Sprintf(format, v...))
}

func (lg *simpleLogger) Panic(v ...any) {
	fmt.Println(append(v))
}
