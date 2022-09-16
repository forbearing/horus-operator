package logger

import (
	"os"
	"strings"

	"github.com/forbearing/horus-operator/pkg/args"
	"github.com/sirupsen/logrus"
)

func Init() {
	logLevel := args.GetLogLevel()
	logFormat := args.GetLogFormat()
	logFile := args.GetLogFile()

	// set log output, default is os.Stdout.
	switch strings.ToUpper(logLevel) {
	case "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
	case "WARN":
		logrus.SetLevel(logrus.WarnLevel)
	case "WARNING":
		logrus.SetLevel(logrus.WarnLevel)
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	case "TRACE":
		logrus.SetLevel(logrus.TraceLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	// set log format, default is text format.
	switch strings.ToUpper(logFormat) {
	case "TEXT":
		logrus.SetFormatter(&logrus.TextFormatter{})
	case "JSON":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	// set log file, default is os.Stdout.
	if len(logFile) == 0 {
		logFile = "/dev/stdout"
	}
	switch logFile {
	case "/dev/stdout":
		logrus.SetOutput(os.Stdout)
	case "/dev/stderr":
		logrus.SetOutput(os.Stderr)
	default:
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
		if err != nil {
			panic(err)
		}
		logrus.SetOutput(file)
	}

	// SetReportCaller sets whether the standard logrus will include the calling
	// method as a field.
	//logrus.SetReportCaller(true)
}

func New() *logrus.Logger {
	logger := logrus.New()

	logLevel := args.GetLogLevel()
	logFormat := args.GetLogFormat()
	logFile := args.GetLogFile()

	// set log output, default is os.Stdout.
	switch strings.ToUpper(logLevel) {
	case "ERROR":
		logger.SetLevel(logrus.ErrorLevel)
	case "WARN":
		logger.SetLevel(logrus.WarnLevel)
	case "WARNING":
		logger.SetLevel(logrus.WarnLevel)
	case "INFO":
		logger.SetLevel(logrus.InfoLevel)
	case "DEBUG":
		logger.SetLevel(logrus.DebugLevel)
	case "TRACE":
		logger.SetLevel(logrus.TraceLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// set log format, default is text format.
	switch strings.ToUpper(logFormat) {
	case "TEXT":
		logger.SetFormatter(&logrus.TextFormatter{})
	case "JSON":
		logger.SetFormatter(&logrus.JSONFormatter{})
	default:
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	// set log file, default is os.Stdout.
	if len(logFile) == 0 {
		logFile = "/dev/stdout"
	}
	switch logFile {
	case "/dev/stdout":
		logger.SetOutput(os.Stdout)
	case "/dev/stderr":
		logger.SetOutput(os.Stderr)
	default:
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
		if err != nil {
			panic(err)
		}
		logger.SetOutput(file)
	}

	//// SetReportCaller sets whether the standard logrus will include the calling
	//// method as a field.
	//logger.SetReportCaller(false)

	return logger
}
