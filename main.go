
package main

import (
    "flag"
    "fmt"
    
//    "github.com/Microsoft/ApplicationInsights-Go/appinsights"
)

var (
    flagLogFormat	string
    flagIkey		string
    flagEndpoint	string
    flagRole		string
    flagInfile		string
    flagTrace		bool
    flagPassStdout	bool
    flagPassStderr	bool
)

var (
    lines = [...]string {
        "192.168.0.1 - - [20/Feb/2017:13:06:06 +0000] \"GET / HTTP/1.1\" 0.000 304 0 \"-\" \"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36\" \"-\"",
        "192.168.0.1 - - [20/Feb/2017:13:06:09 +0000] \"GET / HTTP/1.1\" 0.000 200 612 \"-\" \"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36\" \"-\"",
        "192.168.0.1 - - [20/Feb/2017:13:06:09 +0000] \"GET /favicon.ico HTTP/1.1\" 0.000 404 571 \"http://52.183.35.10/\" \"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36\" \"-\"",
        "192.168.0.1 - - [20/Feb/2017:13:06:14 +0000] \"GET / HTTP/1.1\" 0.000 200 612 \"-\" \"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36\" \"-\"",
        "192.168.0.1 - - [20/Feb/2017:13:06:14 +0000] \"GET /favicon.ico HTTP/1.1\" 0.000 404 571 \"http://52.183.35.10/\" \"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36\" \"-\"",
        "~silly~"}
)

var myLine = "$remote_addr - $remote_user [$time_local] \"$request\" $request_time $status $body_bytes_sent \"$http_referer\" \"$http_user_agent\""

func init() {
    flag.StringVar(&flagIkey, "ikey", "", "ApplicationInsights instrumentation key")
    flag.StringVar(&flagEndpoint, "endpoint", "", "ApplicationInsights ingestion endpoint (optional)")
    flag.StringVar(&flagRole, "role", "", "Telemetry role instance. Defaults to the machine hostname")
    flag.StringVar(&flagLogFormat, "logformat", "", "nginx log format")
    flag.StringVar(&flagInfile, "infile", "", "Input file, or '-' for stdin")
    flag.BoolVar(&flagTrace, "trace", false, "Don't try to parse input, just send as traces")
    flag.BoolVar(&flagPassStdout, "pass", false, "If specified, write log lines to stdout")
    flag.BoolVar(&flagPassStderr, "passerr", false, "If specified, write log lines to stderr")
}

func main() {
    flag.Parse()
    
    logFormat := flagLogFormat
    if logFormat == "" {
        //logFormat = defaultFormat
        logFormat = myLine
    }
    
    parser, err := MakeLogParser(logFormat, false)
    if err != nil {
        fmt.Printf("Error: %s\n", err.Error())
        return
    }
    
    for _, line := range lines {
        m, err := parser.parseLogLine(line)
//        fmt.Printf("Map=%q\n\n", m)
        if err == nil {
            name, _ := parseName(m)
            ts, _ := parseTimestamp(m)
            dur, _ := parseDuration(m)
            code, _ := parseResponseCode(m)
            succ, _ := parseSuccess(m)
            url, _ := parseUrl(m)
            
            fmt.Printf("Request %q\n  * URL = %q\n  * Timestamp = %q\n  * Code = %q\n  * Duration = %q\n  * Success = %q\n\n", name, url, ts, code, dur, succ)
        } else {
            fmt.Printf("error parsing line: %s\n\n", err.Error())
        }
    }
}
