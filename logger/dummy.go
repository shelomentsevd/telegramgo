// +build !debug

package logger

func LogStruct(i interface{}) {}
func Error(err error) {}
func Info(format string, a ...interface{}) {}