
package main

import (
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
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
    flagDebug		bool
    flagQuiet		bool
)

func init() {
    flag.StringVar(&flagIkey, "ikey", "", "ApplicationInsights instrumentation key")
    flag.StringVar(&flagEndpoint, "endpoint", "", "ApplicationInsights ingestion endpoint (optional)")
    flag.StringVar(&flagRole, "role", "", "Telemetry role instance. Defaults to the machine hostname")
    flag.StringVar(&flagLogFormat, "logformat", "", "nginx log format")
    flag.StringVar(&flagInfile, "infile", "", "Input file, or '-' for stdin")
    flag.BoolVar(&flagTrace, "trace", false, "Don't try to parse input, just send as traces")
    flag.BoolVar(&flagPassStdout, "pass", false, "If specified, write log lines to stdout")
    flag.BoolVar(&flagPassStderr, "passerr", false, "If specified, write log lines to stderr")
    flag.BoolVar(&flagDebug, "debug", false, "Show debugging output")
    flag.BoolVar(&flagQuiet, "quiet", false, "Don't write any output messages")
}

func main() {
    flag.Parse()
    
    logFormat := flagLogFormat
    if logFormat == "" {
        logFormat = defaultFormat
    }
    
    if flagDebug {
        log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
    } else {
        log.SetOutput(ioutil.Discard)
    }
    
    msgs := log.New(os.Stderr, "AI-nginx: ", log.Ldate | log.Ltime)
    if flagQuiet {
        msgs.SetOutput(ioutil.Discard)
    }
    
    if flagInfile == "" {
        fmt.Fprintln(os.Stderr, "Must specify input file. See -help for usage.")
        os.Exit(1)
    }
    
    logParser, err := MakeLogParser(logFormat, false)
    if err != nil {
        msgs.Printf("Error initializing log parser: %s\n", err.Error())
        os.Exit(1)
    }
    
    logReader, err := MakeLogReader(flagInfile)
    if err != nil {
        msgs.Printf("Error initializing log reader: %s\n", err.Error())
        os.Exit(1)
    }
    
    signalc := make(chan os.Signal, 2)
    signal.Notify(signalc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
    
    done := make(chan bool)
    go readLoop(logReader, logParser, msgs, done)
    
    for {
        select {
            case sig := <- signalc:
                msgs.Println(sig.String())
                switch sig {
                case syscall.SIGHUP:
                    msgs.Println("Resetting logfile")
                    logReader.Reset()
                case syscall.SIGINT, syscall.SIGTERM:
                    logReader.Close()
                    
                    // Wait for done
                    select {
                        case <- done: break
                        case <- time.After(time.Duration(250 * time.Millisecond)): break
                    }
                    
                    os.Exit(-int(sig.(syscall.Signal)))
                }
            case <- done:
                os.Exit(0)
        }
    }
}

func readLoop(logReader *LogReader, logParser *LogParser, msgs *log.Logger, done chan bool) {
    events := logReader.Events()
    for {
        event := <- events
        if event.data != "" {
            fmt.Printf("Log line: %s\n", event.data)
            m, err := logParser.parseLogLine(event.data)
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
        
        if event.err != nil {
            msgs.Println(event.err.Error())
        }
        
        if event.closed {
            msgs.Println("Input closed.")
            break
        }
    }
    
    done <- true
}

