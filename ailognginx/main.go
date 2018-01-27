package main

import (
	"flag"
	"log"

	"github.com/jjjordanmsft/ApplicationInsights-logforward/common"
)

func main() {
	handler := &NginxHandler{}

	common.InitFlags()
	flag.StringVar(&handler.format, "format", "", "nginx log format (required)")
	flag.Parse()

	common.Start("ailognginx", handler)
}

type NginxHandler struct {
	format string
	msgs   *log.Logger
	parser *LogParser
}

func (handler *NginxHandler) Initialize(msgs *log.Logger) error {
	handler.msgs = msgs

	if handler.format == "" {
		handler.format = defaultFormat
	}

	var err error
	handler.parser, err = NewLogParser(handler.format, newStatusReader())
	return err
}

func (handler *NginxHandler) Receive(line string) error {
	t, err := handler.parser.CreateTelemetry(line)
	if err == nil && t != nil {
		common.Track(t)
	}

	return err
}
