package logutil

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

func getFormattedStackTrace(depth int) string {
	pc := make([]uintptr, depth)
	n := runtime.Callers(6, pc)
	frames := runtime.CallersFrames(pc[:n])

	stackTrace := ""
	lineno := -1
	for {
		frame, more := frames.Next()
		if !more {
			break
		}

		if strings.Contains(frame.File, "logrus") {
			continue
		}

		lineno += 1
		stackTrace += fmt.Sprintf("    #%d %s:%d %s\n", lineno, frame.File,
			frame.Line, frame.Function)
	}

	return stackTrace
}

type MyFormatter struct {
	logrus.TextFormatter
}

func (f *MyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	if entry.Level == logrus.ErrorLevel || entry.Level == logrus.FatalLevel {
		// 获取完整调用栈
		stacktrace := getFormattedStackTrace(15)
		entry.Message += "\n" + stacktrace
	} else if entry.Level == logrus.WarnLevel {
		// warn日志触发频率更高，获取一小部分调用栈即可
		stacktrace := getFormattedStackTrace(8)
		entry.Message += "\n" + stacktrace
	}
	return f.TextFormatter.Format(entry)
}

func Init() {
	logrus.SetFormatter(&MyFormatter{
		TextFormatter: logrus.TextFormatter{
			ForceColors:     true,
			DisableQuote:    true,
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		},
	})
}

func SetLogLevel(level string) {
	level = strings.ToLower(level)
	switch level {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	}
}

func LogError(format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	logrus.Error(err)
	return err
}

func LogWarn(format string, a ...any) {
	logrus.Warnf(format, a...)
}

func SetLogFile(logFile string) error {
	if logFile == "" {
		return nil
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %s", logFile, err)
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	logrus.SetOutput(multiWriter)
	return nil
}
