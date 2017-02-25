
package main

import (
    "flag"
    "log"
    
    "github.com/jjjordanmsft/ApplicationInsights-nginx/common"
)

func main() {
    handler := &NginxHandler{}

    common.InitFlags()
    flag.StringVar(&handler.format, "format", "", "nginx log format")
    flag.BoolVar(&handler.jsonEscape, "jsonescape", false, "whether the nginx log is JSON-escaped")
    flag.Parse()
    
    common.Start("ailognginx", handler)
}

type NginxHandler struct {
    format	string
    jsonEscape	bool
    msgs	*log.Logger
    parser	*LogParser
}

func (handler *NginxHandler) Initialize(msgs *log.Logger) error {
    handler.msgs = msgs
    
    if handler.format == "" {
        handler.format = defaultFormat
    }
    
    var err error
    handler.parser, err = NewLogParser(handler.format, handler.jsonEscape)
    return err
}

func (handler *NginxHandler) Receive(line string) error {
    t, err := handler.parser.CreateTelemetry(line)
    if err == nil && t != nil {
        common.Track(t)
    }
    
    return err
}
