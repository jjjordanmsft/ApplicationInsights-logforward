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
	flag.BoolVar(&handler.noReject, "noreject", false, "don't reject log lines that may not parse perfectly")
	flag.Parse()

	common.Start("ailognginx", handler)
}

type NginxHandler struct {
	format   string
	noReject bool
	msgs     *log.Logger
	parser   *LogParser
}

func (handler *NginxHandler) Initialize(msgs *log.Logger) error {
	handler.msgs = msgs

	if handler.format == "" {
		handler.format = defaultFormat
	}

	var err error
	handler.parser, err = NewLogParser(handler.format, handler.noReject)
	return err
}

func (handler *NginxHandler) Receive(line string) error {
	t, err := handler.parser.CreateTelemetry(line)
	if err == nil && t != nil {
		common.Track(t)
	}

	return err
}
