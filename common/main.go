
package common

import (
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/jjjordanmsft/ApplicationInsights-Go/appinsights"
)

type LogHandler interface {
    Initialize(*log.Logger)	error
    Receive(string)		error
}

var (
    flagIkey		string
    flagEndpoint	string
    flagRole		string
    flagRoleInstance	string
    flagInfile		string
    flagOutfile		string
    flagCustom		customProperties
    flagDebug		bool
    flagQuiet		bool
    
    tclient		appinsights.TelemetryClient
)

func InitFlags() {
    flag.StringVar(&flagIkey, "ikey", "", "ApplicationInsights instrumentation key")
    flag.StringVar(&flagEndpoint, "endpoint", "", "ApplicationInsights ingestion endpoint (optional)")
    flag.StringVar(&flagRole, "role", "", "Telemetry role name. Defaults to the machine hostname")
    flag.StringVar(&flagRoleInstance, "roleinstance", "", "Telemetry role instance. Defaults to the machine hostname")
    flag.StringVar(&flagInfile, "infile", "", "Input file, or '-' for stdin")
    flag.StringVar(&flagOutfile, "outfile", "", "Output file, '-' for stdout, 'stderr' for stderr")
    flag.BoolVar(&flagDebug, "debug", false, "Show debugging output")
    flag.BoolVar(&flagQuiet, "quiet", false, "Don't write any output messages")
    flag.Var(&flagCustom, "custom", "Define custom property like 'key=value'. Can be used multiple times")
}

func Start(name string, logHandler LogHandler) {
    if flagDebug {
        log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
        
        ailistener := appinsights.NewDiagnosticsMessageListener()
        go ailistener.ProcessMessages(writeAiLog)
    } else {
        log.SetOutput(ioutil.Discard)
    }
    
    if flagInfile == "" {
        fmt.Fprintln(os.Stderr, "Must specify input file. See -help for usage.")
        os.Exit(1)
    }
    
    hostname, _ := os.Hostname()
    if flagRole == "" {
        flagRole = hostname
    }
    
    if flagRoleInstance == "" {
        flagRoleInstance = hostname
    }
    
    if flagIkey == "" {
        fmt.Fprintln(os.Stderr, "Must specify instrumentation key. See -help for usage.")
        os.Exit(1)
    }
    
    tconfig := appinsights.NewTelemetryConfiguration(flagIkey)
    if flagEndpoint != "" {
        tconfig.EndpointUrl = flagEndpoint
    }
    
    tclient = appinsights.NewTelemetryClientFromConfig(tconfig)
    
    msgs := log.New(os.Stderr, fmt.Sprintf("%s: ", name), log.Ldate | log.Ltime)
    if flagQuiet {
        msgs.SetOutput(ioutil.Discard)
    }
    
    logReader, err := MakeLogReader(flagInfile)
    if err != nil {
        msgs.Printf("Error initializing log reader: %s\n", err.Error())
        os.Exit(1)
    }
    
    err = logHandler.Initialize(msgs)
    if err != nil {
        msgs.Printf("Error initializing log handler: %s\n", err.Error())
        os.Exit(1)
    }
    
    signalc := make(chan os.Signal, 2)
    signal.Notify(signalc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
    
    done := make(chan bool)
    go readLoop(logReader, logHandler, msgs, done)
    
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

func readLoop(logReader *LogReader, logHandler LogHandler, msgs *log.Logger, done chan bool) {
    events := logReader.Events()
    for {
        event := <- events
        if event.data != "" {
            log.Printf("Log line: %s", event.data)
            err := logHandler.Receive(event.data)
            if err != nil {
                log.Printf("Error was: %s", err.Error())
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

func Track(t appinsights.Telemetry) {
    if t != nil {
        cloud := t.Context().Cloud()
        cloud.SetRoleName(flagRole)
        cloud.SetRoleInstance(flagRoleInstance)
        if flagCustom != nil {
            for k, v := range flagCustom {
                t.SetProperty(k, v)
            }
        }
        
        tclient.Track(t)
    }
}

func writeAiLog(msg string) {
    log.Println(msg)
}
