package logger

import "log"

type Logging interface {
	Info(arg ...interface{})
	Infof(format string, args ...interface{})

	Warn(arg ...interface{})
	Warnf(format string, args ...interface{})

	Error(arg ...interface{})
	Errorf(format string, args ...interface{})

	Debug(arg ...interface{})
	Debugf(format string, args ...interface{})
}

var Logger Logging = &DefaultLogger{}

func Info(arg ...interface{}) {
	Logger.Info(arg)
}
func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args)
}

func Warn(arg ...interface{}) {
	Logger.Warn(arg)
}
func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args)
}

func Error(arg ...interface{}) {
	Logger.Error(arg)
}
func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args)
}

func Debug(arg ...interface{}) {
	Logger.Debug(arg)
}
func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args)
}

var _Logging = (*DefaultLogger)(nil)

type DefaultLogger struct {
}

func (*DefaultLogger) Info(arg ...interface{}) {
	args := append([]interface{}{"[Info] "}, arg...)
	log.Println(args...)
}
func (*DefaultLogger) Infof(format string, args ...interface{}) {
	log.Println("[Info] "+format+"\n", args)
}

func (*DefaultLogger) Warn(arg ...interface{}) {
	args := append([]interface{}{"[Warn] "}, arg...)
	log.Println(args...)
}
func (*DefaultLogger) Warnf(format string, args ...interface{}) {
	log.Printf("[Warn] "+format+"\n", args)
}

func (*DefaultLogger) Error(arg ...interface{}) {
	args := append([]interface{}{"[Error] "}, arg...)
	log.Println(args...)
}
func (*DefaultLogger) Errorf(format string, args ...interface{}) {
	log.Printf("[Error] "+format+"\n", args)
}

func (*DefaultLogger) Debug(arg ...interface{}) {
	args := append([]interface{}{"[Debug] "}, arg...)
	log.Println(args...)
}
func (*DefaultLogger) Debugf(format string, args ...interface{}) {
	log.Printf("[Debug] "+format+"\n", args)
}
