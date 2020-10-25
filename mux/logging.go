package mux

import (
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func init() {
	out := ioutil.Discard
	if os.Getenv("PROTOCOL_DEBUG") == "1" {
		out = os.Stdout
	}
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	log = &logrus.Logger{
		Level:     logrus.DebugLevel,
		Out:       out,
		Formatter: formatter,
	}
}
