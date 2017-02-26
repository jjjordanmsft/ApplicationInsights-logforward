
package main

import (
    "bytes"
    "flag"
    "fmt"
    "log"
    "strings"
    "time"
    
    "github.com/jjjordanmsft/ApplicationInsights-Go/appinsights"
    "github.com/jjjordanmsft/ApplicationInsights-logforward/common"
)

var (
    severity = map[string]appinsights.SeverityLevel{
        "verbose": appinsights.Verbose,
        "information": appinsights.Information,
        "info": appinsights.Information,
        "warning": appinsights.Warning,
        "warn": appinsights.Warning,
        "error": appinsights.Error,
        "err": appinsights.Error,
        "critical": appinsights.Critical,
        "crit": appinsights.Critical,
    }
)

func main() {
    handler := &TraceHandler{}
    
    common.InitFlags()
    flag.Var(&handler.filterInclude, "include", "Include lines that match this regex")
    flag.Var(&handler.filterExclude, "exclude", "Exclude lines that match this regex")
    flag.IntVar(&handler.batchTime, "batch", 0, "Batch output for n seconds and send as a single trace")
    flag.StringVar(&handler.sevstring, "severity", "Information", "Severity level in trace telemetry: Verbose, Information, Warning, Error, Critical")
    flag.Parse()
    
    common.Start("ailogtrace", handler)
}

type TraceHandler struct {
    msgs		*log.Logger
    filterInclude	regexpList
    filterExclude	regexpList
    batchTime		int
    channel		chan string
    sevstring		string
    severity		appinsights.SeverityLevel
}

func (handler *TraceHandler) Initialize(msgs *log.Logger) error {
    handler.msgs = msgs
    
    if val, ok := severity[strings.ToLower(handler.sevstring)]; ok {
        handler.severity = val
    } else {
        return fmt.Errorf("Invalid severity level, must be one of: verbose, information, warning, error, critical")
    }
    
    handler.channel = make(chan string)
    if handler.batchTime > 0 {
        go handler.batchMessages()
    } else {
        go handler.passMessages()
    }
    
    return nil
}

func (handler *TraceHandler) Receive(line string) error {
    if handler.filterInclude.MatchAny(line, true) && !handler.filterExclude.MatchAny(line, false) {
        handler.channel <- line
    } else {
        log.Printf("Line didn't pass regexps: %s", line)
    }
    
    return nil
}

func (handler *TraceHandler) batchMessages() {
    var buf bytes.Buffer
    
    for {
        line := <- handler.channel
        buf.WriteString(line)
        
        timeout := time.After(time.Duration(handler.batchTime) * time.Second)
wait:   for {
            select {
            case line = <- handler.channel:
                buf.WriteString(line)
            case _ = <- timeout:
                t := appinsights.NewTraceTelemetry(buf.String(), handler.severity)
                common.Track(t)
                buf.Reset()
                break wait
            }
        }
    }
}

func (handler *TraceHandler) passMessages() {
    for {
        line := <- handler.channel
        t := appinsights.NewTraceTelemetry(strings.TrimRight(line, "\r\n"), handler.severity)
        common.Track(t)
    }
}
