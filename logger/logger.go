// +build debug

package logger

import (
	"os"
	"fmt"
	"runtime"
	"encoding/json"
)

var fileLog * os.File

func function() string {
	pc := make([]uintptr, 10)
	runtime.Callers(3, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])
	return fmt.Sprintf("%s:%d %s\n", file, line, f.Name())
}

func init() {
	file, err := os.OpenFile("./debug-logs.txt", os.O_WRONLY | os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("Can't open debug-logs.txt file!")
		os.Exit(2)
	}
	fileLog = file
}

func Error(err error) {
	log := fmt.Sprintf("%s\nERROR: %s\n", function(), err)
	fileLog.WriteString(log)
}

func LogStruct(i interface{}) {
	bytes, err := json.Marshal(i)
	if err != nil {
		fmt.Printf("Failed to serialize interface to json in function %s\n", function())
	}
	log := fmt.Sprintf("%s\nSTRUCT:%s\n", function(), string(bytes))
	fileLog.WriteString(log)
}

func Info(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	log := fmt.Sprintf("%s\nINFO:%s\n", function(), str)
	fileLog.WriteString(log)
}