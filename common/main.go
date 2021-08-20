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

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)

type LogHandler interface {
	Initialize(*log.Logger) error
	Receive(string) error
}

var (
	flagIkey         string
	flagEndpoint     string
	flagRole         string
	flagRoleInstance string
	flagInfile       string
	flagOutfile      string
	flagCustom       customProperties
	flagFlushWait    time.Duration
	flagDebug        bool
	flagQuiet        bool

	tclient appinsights.TelemetryClient
)

func InitFlags() {
	flag.StringVar(&flagIkey, "ikey", "", "ApplicationInsights instrumentation key (required)")
	flag.StringVar(&flagEndpoint, "endpoint", "", "ApplicationInsights ingestion endpoint")
	flag.StringVar(&flagRole, "role", "", "Telemetry role name. Defaults to the machine hostname")
	flag.StringVar(&flagRoleInstance, "roleinstance", "", "Telemetry role instance. Defaults to the machine hostname")
	flag.StringVar(&flagInfile, "in", "", "Input file, or '-' for stdin (required)")
	flag.StringVar(&flagOutfile, "out", "", "Output file, '-' for stdout, 'stderr' for stderr")
	flag.DurationVar(&flagFlushWait, "flush", 3*time.Second, "Timeout to flush telemetry when shutting down")
	flag.BoolVar(&flagDebug, "debug", false, "Show debugging output")
	flag.BoolVar(&flagQuiet, "quiet", false, "Don't write any output messages")
	flag.Var(&flagCustom, "custom", "Include custom property in telemetry like 'key=value'. Can be used multiple times")
}

func Start(name string, logHandler LogHandler) {
	if flagDebug {
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		appinsights.NewDiagnosticsMessageListener(writeAiLog)
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

	// Propagate custom flags to common properties
	for k, v := range flagCustom {
		tclient.Context().CommonProperties[k] = v
	}

	msgs := log.New(os.Stderr, fmt.Sprintf("%s: ", name), log.Ldate|log.Ltime)
	if flagQuiet {
		msgs.SetOutput(ioutil.Discard)
	}

	cloud := tclient.Context().Tags.Cloud()
	cloud.SetRole(flagRole)
	cloud.SetRoleInstance(flagRoleInstance)

	var logWriter *LogWriter
	if flagOutfile != "" {
		var err error
		logWriter, err = NewLogWriter(flagOutfile)
		if err != nil {
			msgs.Printf("Error initializing log writer: %s\n", err.Error())
			os.Exit(1)
		}
	} else {
		logWriter = NewNilLogWriter()
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
	go readLoop(logReader, logWriter, logHandler, msgs, done)

	for {
		select {
		case sig := <-signalc:
			msgs.Println(sig.String())
			switch sig {
			case syscall.SIGHUP:
				msgs.Println("Resetting logfile")
				logReader.Reset()
			case syscall.SIGINT, syscall.SIGTERM:
				logReader.Close()

				// Begin flush of AI client
				tclient.Channel().Flush()

				// Wait for done
				select {
				case <-done:
					break
				case <-time.After(time.Duration(250 * time.Millisecond)):
					break
				}

				logWriter.Close()

				// Close down telemetry channel and try to send out any remaining events.
				select {
				case <-tclient.Channel().Close(flagFlushWait):
					break
				case <-time.After(flagFlushWait):
					break
				}

				os.Exit(-int(sig.(syscall.Signal)))
			}
		case <-done:
			// Flush out events and close down AI sender.
			select {
			case <-tclient.Channel().Close(flagFlushWait):
				break
			case <-time.After(flagFlushWait):
				break
			}

			os.Exit(0)
		}
	}
}

func readLoop(logReader *LogReader, logWriter *LogWriter, logHandler LogHandler, msgs *log.Logger, done chan bool) {
main:
	for {
		select {
		case event := <-logReader.events:
			if event.data != "" {
				if logWriter != nil {
					logWriter.Write(event.data)
				}

				err := logHandler.Receive(event.data)
				if err != nil {
					msgs.Println(fmt.Sprintf("Error processing log line. Error: %s Original log line: %s", err.Error(), event.data))
				}
			}

			if event.err != nil {
				msgs.Println(event.err.Error())
			}

			if event.closed {
				msgs.Println("Input closed.")
				break main
			}
		case event := <-logWriter.events:
			if event.err != nil {
				msgs.Printf("Log output encountered error: %s", event.err.Error())
			}

			if event.closed {
				msgs.Println("Log output closed. Aborting.")
				break main
			}
		}
	}

	done <- true
}

func Track(t appinsights.Telemetry) {
	if t != nil {
		tclient.Track(t)
	}
}

func writeAiLog(msg string) error {
	log.Println(msg)
	return nil
}
