package micro

type Logger interface {
	Debug(...any)
	Info(...any)
	Infof(string, ...any)
	Warn(...any)
	Error(...any)
	Errorf(string, ...any)
	Panic(...any)
}
