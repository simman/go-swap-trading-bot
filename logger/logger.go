package logger

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func init() {
	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)
	l.SetOutput(io.MultiWriter(os.Stdout))
	l.SetReportCaller(true)

	l.SetFormatter(&logrus.TextFormatter{})

	Log = l
}
